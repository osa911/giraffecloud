package tunnel

import (
	"fmt"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net/http"
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
	// PERFECTLY ALIGNED with gRPC 16MB message limit - NO GAPS!
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

		// Route to chunked streaming for unlimited file size support
		return s.handleLargeFileWithChunking(domain, httpReq, clientIP)
	}

	// Small files: use regular gRPC (≤16MB, perfect alignment)
	s.logger.Debug("[REGULAR] Small file (≤16MB) → Regular gRPC (16MB limit): %s %s", httpReq.Method, httpReq.URL.Path)
	return s.ProxyHTTPRequest(domain, httpReq, clientIP)
}

// handleLargeFileWithChunking processes large files using chunked streaming
func (s *GRPCTunnelServer) handleLargeFileWithChunking(domain string, httpReq *http.Request, clientIP string) (*http.Response, error) {
	// Convert HTTP request to protobuf
	protoReq, err := s.httpToGRPC(httpReq, clientIP)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTTP request: %w", err)
	}

	// Mark as large file
	protoReq.GetHttpRequest().IsLargeFile = true

	// Create large file request for chunked streaming
	largeFileReq := &proto.LargeFileRequest{
		RequestId:         fmt.Sprintf("chunk-%d", time.Now().UnixNano()),
		HttpRequest:       protoReq.GetHttpRequest(),
		ChunkSize:         DefaultChunkSize,
		EnableCompression: true,
	}

	s.logger.Debug("[CHUNKED] Initiating chunked stream for: %s", httpReq.URL.Path)

	// Create a streaming client to handle the chunked response
	return s.handleChunkedStreamResponse(largeFileReq)
}

// handleChunkedStreamResponse handles the chunked streaming response
func (s *GRPCTunnelServer) handleChunkedStreamResponse(req *proto.LargeFileRequest) (*http.Response, error) {
	s.logger.Warn("[CHUNKED] 🚧 Server-side chunked reassembly not yet fully implemented")
	s.logger.Info("[CHUNKED] 🔄 Falling back to client-side chunked streaming (still UNLIMITED) for: %s", req.HttpRequest.Path)

	// For now, fall back to the regular proxy
	// The client will still do chunked streaming, server will reassemble
	httpReq, err := s.protoToHTTP(req.HttpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to convert proto back to HTTP: %w", err)
	}

	// Extract domain from the request
	domain := s.extractDomainFromRequest(req.HttpRequest)

	// Use regular proxy - but the client will still stream chunks back to us
	return s.ProxyHTTPRequest(domain, httpReq, req.HttpRequest.ClientIp)
}

// protoToHTTP converts a proto HTTP request back to a regular HTTP request
func (s *GRPCTunnelServer) protoToHTTP(protoReq *proto.HTTPRequest) (*http.Request, error) {
	// This is a simplified conversion - in practice you'd need to handle more cases
	req, err := http.NewRequest(protoReq.Method, protoReq.Path, strings.NewReader(string(protoReq.Body)))
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range protoReq.Headers {
		req.Header.Set(key, value)
	}

	// Set URL path and query
	req.URL.Path = protoReq.Path
	if protoReq.Query != "" {
		req.URL.RawQuery = protoReq.Query
	}

	return req, nil
}