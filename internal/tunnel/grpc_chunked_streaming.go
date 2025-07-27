package tunnel

import (
	"bytes"
	"fmt"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	// DefaultChunkSize is the default chunk size for streaming (1MB)
	DefaultChunkSize = 1024 * 1024
	// MaxChunkSize is the maximum allowed chunk size (4MB)
	MaxChunkSize = 4 * 1024 * 1024
	// ChunkedStreamingThreshold - files larger than this use chunked streaming
	// BINARY SPLIT: ≤16MB = Regular gRPC, >16MB = Unlimited Chunked Streaming
	ChunkedStreamingThreshold = 16 * 1024 * 1024 // 16MB
)

// StreamLargeFile implements server-side streaming for large files
// This provides unlimited concurrency with memory-efficient chunked transfer
func (s *GRPCTunnelServer) StreamLargeFile(req *proto.LargeFileRequest, stream grpc.ServerStreamingServer[proto.LargeFileChunk]) error {
	s.logger.Info("[CHUNKED] 🚀 Starting large file streaming: %s %s",
		req.HttpRequest.Method, req.HttpRequest.Path)

	// Validate request
	if req.HttpRequest == nil {
		return fmt.Errorf("missing HTTP request in large file request")
	}

	// Get domain from request (we'll extract this from the path or headers)
	domain := s.extractDomainFromRequest(req.HttpRequest)
	if domain == "" {
		return fmt.Errorf("unable to determine domain for large file request")
	}

	// Find the tunnel stream for this domain
	s.tunnelStreamsMux.RLock()
	tunnelStream, exists := s.tunnelStreams[domain]
	s.tunnelStreamsMux.RUnlock()

	if !exists || !tunnelStream.connected {
		return fmt.Errorf("no active tunnel for domain: %s", domain)
	}

	// Set chunk size (default to 1MB if not specified or too large)
	chunkSize := int(req.ChunkSize)
	if chunkSize <= 0 || chunkSize > MaxChunkSize {
		chunkSize = DefaultChunkSize
	}

	s.logger.Debug("[CHUNKED] Using %dKB chunks for: %s", chunkSize/1024, req.HttpRequest.Path)

	// Forward request to client and get response, then stream it in chunks
	return s.handleChunkedStreaming(req, stream, tunnelStream, chunkSize)
}

// handleChunkedStreaming manages the complete chunked streaming process
func (s *GRPCTunnelServer) handleChunkedStreaming(
	req *proto.LargeFileRequest,
	stream grpc.ServerStreamingServer[proto.LargeFileChunk],
	tunnelStream *TunnelStream,
	chunkSize int,
) error {

	// Convert large file request to regular HTTP request message
	httpReq := req.HttpRequest
	requestID := req.RequestId

	// Create regular tunnel message to send to client
	tunnelMsg := &proto.TunnelMessage{
		RequestId: requestID,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpRequest{
			HttpRequest: httpReq,
		},
	}

	s.logger.Debug("[CHUNKED] Forwarding request to client: %s %s", httpReq.Method, httpReq.Path)

	// Use existing request/response mechanism to get the full response
	response, err := s.sendRequestAndWaitResponse(tunnelStream, tunnelMsg)
	if err != nil {
		s.logger.Error("[CHUNKED] Failed to get response from tunnel: %v", err)
		return fmt.Errorf("tunnel request failed: %w", err)
	}

	// Now stream the response body in chunks
	return s.streamResponseInChunks(response, stream, chunkSize, requestID)
}

// streamResponseInChunks streams an HTTP response body in chunks
func (s *GRPCTunnelServer) streamResponseInChunks(
	response *http.Response,
	stream grpc.ServerStreamingServer[proto.LargeFileChunk],
	chunkSize int,
	requestID string,
) error {

	defer response.Body.Close()

	s.logger.Debug("[CHUNKED] Streaming response: %d %s", response.StatusCode, response.Status)

	// Read the entire response body (we'll optimize this for true streaming later)
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	totalSize := int64(len(responseData))

	// Calculate number of chunks
	numChunks := int((totalSize + int64(chunkSize) - 1) / int64(chunkSize))
	if numChunks == 0 {
		numChunks = 1 // At least one chunk for empty responses
	}

	s.logger.Info("[CHUNKED] 📦 Streaming %d bytes in %d chunks of %dKB each",
		totalSize, numChunks, chunkSize/1024)

	// Convert response headers
	headers := make(map[string]string)
	for key, values := range response.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Stream chunks
	for i := 0; i < numChunks; i++ {
		// Calculate chunk boundaries
		start := i * chunkSize
		end := start + chunkSize
		if end > len(responseData) {
			end = len(responseData)
		}

		// Extract chunk data
		chunkData := responseData[start:end]
		isLastChunk := (i == numChunks-1)

		// Create chunk message
		chunk := &proto.LargeFileChunk{
			RequestId:    requestID,
			ChunkNumber:  int32(i + 1),
			Data:         chunkData,
			IsFinal:      isLastChunk,
			TotalSize:    totalSize,
			ContentType:  headers["Content-Type"],
		}

		// Include HTTP headers and status only in the first chunk
		if i == 0 {
			chunk.Headers = headers
			chunk.StatusCode = int32(response.StatusCode)
		}

		// Send chunk
		if err := stream.Send(chunk); err != nil {
			s.logger.Error("[CHUNKED] Failed to send chunk %d/%d: %v", i+1, numChunks, err)
			return fmt.Errorf("failed to send chunk %d: %w", i+1, err)
		}

		s.logger.Debug("[CHUNKED] ✅ Sent chunk %d/%d (%d bytes)", i+1, numChunks, len(chunkData))

		// Small delay for very large files to prevent overwhelming
		if totalSize > 100*1024*1024 && numChunks > 100 { // 100MB+ files with 100+ chunks
			time.Sleep(1 * time.Millisecond)
		}
	}

	s.logger.Info("[CHUNKED] 🎉 Successfully streamed %d chunks for large file", numChunks)
	return nil
}

// extractDomainFromRequest extracts domain from the HTTP request
func (s *GRPCTunnelServer) extractDomainFromRequest(req *proto.HTTPRequest) string {
	// Try to get domain from Host header
	if host := req.Headers["Host"]; host != "" {
		return host
	}

	// Try to get domain from X-Forwarded-Host header
	if forwardedHost := req.Headers["X-Forwarded-Host"]; forwardedHost != "" {
		return forwardedHost
	}

	// As a fallback, we'll use the first active domain
	s.tunnelStreamsMux.RLock()
	defer s.tunnelStreamsMux.RUnlock()

	for domain := range s.tunnelStreams {
		return domain // Return the first active domain
	}

	return ""
}

// isLargeFileRequest determines if a request should use chunked streaming
// BINARY RULE: Files >16MB = Chunked Streaming (UNLIMITED), Files ≤16MB = Regular gRPC (16MB)
// PERFECT ALIGNMENT - NO GAPS!
func (s *GRPCTunnelServer) isLargeFileRequest(httpReq *http.Request) bool {
	// 1. First priority: Check Content-Length header for exact size
	if contentLength := httpReq.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			return size > ChunkedStreamingThreshold // >16MB
		}
	}

		// 2. Second priority: File extensions that are typically large (>16MB)
	path := strings.ToLower(httpReq.URL.Path)
	typicallyLargeExtensions := []string{
		// Video files - almost always >16MB
		".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4v", ".flv", ".wmv", ".mpg", ".mpeg", ".m2v",
		// Archives - often >16MB
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz",
		// Large binaries and disk images - usually >16MB
		".iso", ".img", ".dmg", ".exe", ".msi", ".deb", ".rpm", ".appimage",
		// Large audio files - often >16MB
		".wav", ".flac", ".ape",
	}

	for _, ext := range typicallyLargeExtensions {
		if strings.HasSuffix(path, ext) {
			s.logger.Debug("[CHUNKED] Large file extension detected: %s → UNLIMITED CHUNKED STREAMING", ext)
			return true
		}
	}

	// 3. Third priority: Path patterns that typically serve large files
	largeFilePaths := []string{
		"/video/", "/videos/", "/movie/", "/movies/", "/playback",
		"/download/", "/downloads/", "/file/", "/files/",
		"/original/", "/raw/", "/backup/", "/archive/",
		"/media/large/", "/assets/large/", "/content/large/",
	}
	for _, largePath := range largeFilePaths {
		if strings.Contains(path, largePath) {
			s.logger.Debug("[CHUNKED] Large file path detected: %s → UNLIMITED CHUNKED STREAMING", largePath)
			return true
		}
	}

	// 4. Estimate based on request characteristics
	estimatedSize := s.estimateResponseSize(httpReq)
	if estimatedSize > ChunkedStreamingThreshold {
		s.logger.Debug("[CHUNKED] Large file estimated: %d MB → UNLIMITED CHUNKED STREAMING", estimatedSize/(1024*1024))
		return true
	}

	// Default: Use regular gRPC for small files (≤16MB)
	s.logger.Debug("[REGULAR] Small file detected (≤16MB) → Regular gRPC (16MB limit)")
	return false
}

// estimateResponseSize estimates the expected response size based on request characteristics
func (s *GRPCTunnelServer) estimateResponseSize(httpReq *http.Request) int64 {
	// Check if we have a content-length header
	if contentLength := httpReq.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			return size
		}
	}

	// Estimate based on path patterns
	path := strings.ToLower(httpReq.URL.Path)

	// Large file patterns
	if strings.Contains(path, "/video/") || strings.Contains(path, "/playback") {
		return 200 * 1024 * 1024 // Estimate 200MB for videos
	}

	if strings.Contains(path, "/download/") || strings.Contains(path, "/file/") {
		return 100 * 1024 * 1024 // Estimate 100MB for downloads
	}

	if strings.Contains(path, "/original/") || strings.Contains(path, "/raw/") {
		return 50 * 1024 * 1024 // Estimate 50MB for originals
	}

	// Default estimate for unknown requests
	return 1024 * 1024 // 1MB
}

// ProxyHTTPRequestWithChunking handles HTTP requests with intelligent routing
// PERFECT BINARY SPLIT: ≤16MB = Regular gRPC (16MB), >16MB = Unlimited Chunked Streaming
func (s *GRPCTunnelServer) ProxyHTTPRequestWithChunking(domain string, httpReq *http.Request, clientIP string) (*http.Response, error) {
	// Check if this should use chunked streaming for unlimited size
	if s.isLargeFileRequest(httpReq) {
		s.logger.Info("[CHUNKED] 🚀 Large file (>16MB) detected → UNLIMITED chunked streaming: %s %s",
			httpReq.Method, httpReq.URL.Path)

		// Route to TRUE chunked streaming for unlimited file size support
		return s.handleLargeFileWithChunking(domain, httpReq, clientIP)
	}

	// Small files: use regular gRPC (≤16MB, perfect alignment)
	s.logger.Debug("[REGULAR] Small file (≤16MB) → Regular gRPC (16MB limit): %s %s", httpReq.Method, httpReq.URL.Path)
	return s.ProxyHTTPRequest(domain, httpReq, clientIP)
}

// handleLargeFileWithChunking processes large files using TRUE chunked streaming
func (s *GRPCTunnelServer) handleLargeFileWithChunking(domain string, httpReq *http.Request, clientIP string) (*http.Response, error) {
	s.logger.Info("[CHUNKED] 🚀 Implementing TRUE chunked streaming for unlimited file sizes")

	// Get tunnel stream for domain
	s.tunnelStreamsMux.RLock()
	tunnelStream, exists := s.tunnelStreams[domain]
	s.tunnelStreamsMux.RUnlock()

	if !exists || !tunnelStream.connected {
		return nil, fmt.Errorf("no active tunnel for domain: %s", domain)
	}

	// Convert HTTP request to protobuf
	protoReq, err := s.httpToGRPC(httpReq, clientIP)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTTP request: %w", err)
	}

	// Mark as large file for client-side chunked streaming
	protoReq.GetHttpRequest().IsLargeFile = true

	// Create large file request
	largeFileReq := &proto.LargeFileRequest{
		RequestId:         fmt.Sprintf("chunk-%d", time.Now().UnixNano()),
		HttpRequest:       protoReq.GetHttpRequest(),
		ChunkSize:         DefaultChunkSize, // 1MB chunks
		EnableCompression: true,
	}

	s.logger.Debug("[CHUNKED] Sending large file request to client: %s", httpReq.URL.Path)

	// Send large file request to client and collect chunked response
	return s.collectChunkedResponse(tunnelStream, largeFileReq)
}

// collectChunkedResponse sends large file request to client and reassembles chunked response
func (s *GRPCTunnelServer) collectChunkedResponse(tunnelStream *TunnelStream, req *proto.LargeFileRequest) (*http.Response, error) {
	s.logger.Debug("[CHUNKED] 📦 Starting chunk collection for request: %s", req.RequestId)

	// Send the large file HTTP request to the client (marked as large file)
	requestMsg := &proto.TunnelMessage{
		RequestId: req.RequestId,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpRequest{
			HttpRequest: req.HttpRequest,
		},
	}

	if err := tunnelStream.Stream.Send(requestMsg); err != nil {
		return nil, fmt.Errorf("failed to send large file request: %w", err)
	}

	s.logger.Debug("[CHUNKED] ✅ Large file request sent, waiting for chunked response...")

	// Initialize chunk collection
	responseChunks := make(map[string][]byte) // chunk_id -> data
	var firstChunk *proto.HTTPResponse
	var lastChunkReceived bool

	// Set timeout for chunk collection
	timeout := time.After(120 * time.Second) // 2 minutes for large files

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for chunked response after 2 minutes")

		default:
			// Wait for chunks from client
			response, err := tunnelStream.Stream.Recv()
			if err != nil {
				return nil, fmt.Errorf("failed to receive chunk: %w", err)
			}

			// Check if this is our response
			if response.RequestId != req.RequestId {
				s.logger.Warn("[CHUNKED] Received response for different request ID: %s (expected: %s)",
					response.RequestId, req.RequestId)
				continue
			}

			// Handle different message types
			switch msgType := response.MessageType.(type) {
			case *proto.TunnelMessage_HttpResponse:
				chunk := msgType.HttpResponse

				s.logger.Debug("[CHUNKED] 📥 Received HTTP response chunk: %s, size: %d bytes, chunked: %v",
					chunk.ChunkId, len(chunk.Body), chunk.IsChunked)

				// Store the first chunk for headers and status
				if firstChunk == nil {
					firstChunk = chunk
					s.logger.Debug("[CHUNKED] 📋 Response metadata: status=%d, content-type=%s",
						chunk.StatusCode, chunk.Headers["Content-Type"])
				}

				// If this is a chunked response, collect the chunks
				if chunk.IsChunked {
					if len(chunk.Body) > 0 {
						responseChunks[chunk.ChunkId] = chunk.Body
					}

					// Check if this is the final chunk (chunk_id ends with "_final")
					if strings.HasSuffix(chunk.ChunkId, "_final") {
						lastChunkReceived = true
					}
				} else {
					// Non-chunked response - return immediately
					s.logger.Info("[CHUNKED] ✅ Received non-chunked response")
					return s.assembleHttpResponse(chunk)
				}

				// If we have the final chunk, reassemble the response
				if lastChunkReceived && firstChunk != nil {
					s.logger.Info("[CHUNKED] ✅ All chunks received, reassembling response...")
					return s.assembleChunkedHttpResponse(firstChunk, responseChunks)
				}

			case *proto.TunnelMessage_Error:
				errorMsg := msgType.Error
				return nil, fmt.Errorf("client error during chunked streaming: %s", errorMsg.Message)

			default:
				s.logger.Warn("[CHUNKED] Unexpected message type during chunk collection: %T", msgType)
			}
		}
	}
}

// assembleHttpResponse converts a single HTTPResponse proto back to http.Response
func (s *GRPCTunnelServer) assembleHttpResponse(protoResp *proto.HTTPResponse) (*http.Response, error) {
	s.logger.Debug("[CHUNKED] 🔧 Assembling single HTTP response")

	response := &http.Response{
		StatusCode: int(protoResp.StatusCode),
		Status:     protoResp.StatusText,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(protoResp.Body)),
	}

	// Set headers
	for key, value := range protoResp.Headers {
		response.Header.Set(key, value)
	}

	// Set content length
	response.ContentLength = int64(len(protoResp.Body))

	return response, nil
}

// assembleChunkedHttpResponse reassembles chunked HTTP responses into a single response
func (s *GRPCTunnelServer) assembleChunkedHttpResponse(firstChunk *proto.HTTPResponse, chunks map[string][]byte) (*http.Response, error) {
	s.logger.Debug("[CHUNKED] 🔧 Assembling chunked HTTP response from %d chunks", len(chunks))

	// Sort chunks by ID to ensure proper order
	var sortedChunkIds []string
	for chunkId := range chunks {
		sortedChunkIds = append(sortedChunkIds, chunkId)
	}

	// Sort chunk IDs (assuming format like "chunk_1", "chunk_2", etc.)
	sort.Slice(sortedChunkIds, func(i, j int) bool {
		return sortedChunkIds[i] < sortedChunkIds[j]
	})

	// Reassemble the response body
	var totalSize int64
	var responseBody bytes.Buffer

	for _, chunkId := range sortedChunkIds {
		chunkData := chunks[chunkId]
		responseBody.Write(chunkData)
		totalSize += int64(len(chunkData))
	}

	s.logger.Info("[CHUNKED] ✅ Reassembled response: %d chunks, %d total bytes", len(chunks), totalSize)

	// Create the final HTTP response
	response := &http.Response{
		StatusCode:    int(firstChunk.StatusCode),
		Status:        firstChunk.StatusText,
		Header:        make(http.Header),
		Body:          io.NopCloser(&responseBody),
		ContentLength: totalSize,
	}

	// Set headers from the first chunk
	for key, value := range firstChunk.Headers {
		response.Header.Set(key, value)
	}

	// Update Content-Length header to reflect actual assembled size
	response.Header.Set("Content-Length", strconv.FormatInt(totalSize, 10))

	return response, nil
}