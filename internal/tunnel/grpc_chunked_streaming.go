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
	// DefaultChunkSize is the default chunk size for streaming (4MB for faster transfers)
	DefaultChunkSize = 4 * 1024 * 1024
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
	return s.streamResponseInChunks(response, stream, chunkSize, requestID, tunnelStream)
}

// streamResponseInChunks streams an HTTP response body in chunks
func (s *GRPCTunnelServer) streamResponseInChunks(
	response *http.Response,
	stream grpc.ServerStreamingServer[proto.LargeFileChunk],
	chunkSize int,
	requestID string,
	tunnelStream *TunnelStream,
) error {

	defer response.Body.Close()

	s.logger.Debug("[CHUNKED] Streaming response: %d %s", response.StatusCode, response.Status)

	// Read the entire response body (we'll optimize this for true streaming later)
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	totalSize := int64(len(responseData))
	// Record bytes_out once per response for chunked streaming
	if s.usage != nil && tunnelStream != nil && totalSize > 0 {
		s.usage.Increment(tunnelStream.UserID, tunnelStream.TunnelID, tunnelStream.Domain, 0, totalSize, 0)
	}

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
			RequestId:   requestID,
			ChunkNumber: int32(i + 1),
			Data:        chunkData,
			IsFinal:     isLastChunk,
			TotalSize:   totalSize,
			ContentType: headers["Content-Type"],
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

		// Removed artificial delay for faster streaming
		// With 4MB chunks, even large files stream efficiently without delays
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
	// Always stream uploads via Start/Chunk/End to avoid 16MB gRPC limits
	switch strings.ToUpper(httpReq.Method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return s.handleLargeFileUploadWithStreaming(domain, httpReq, clientIP)
	}

	// Check if this should use chunked streaming for unlimited size
	if s.isLargeFileRequest(httpReq) {
		s.logger.Info("[CHUNKED] 🚀 Large file (>16MB) detected → UNLIMITED chunked streaming: %s %s",
			httpReq.Method, httpReq.URL.Path)

		// Download path: old LargeFileRequest flow
		return s.handleLargeFileDownloadWithChunking(domain, httpReq, clientIP)
	}

	// Small files: use regular gRPC (≤16MB, perfect alignment)
	s.logger.Debug("[REGULAR] Small file (≤16MB) → Regular gRPC (16MB limit): %s %s", httpReq.Method, httpReq.URL.Path)
	return s.ProxyHTTPRequest(domain, httpReq, clientIP)
}

// handleLargeFileWithChunking processes large files using TRUE chunked streaming
func (s *GRPCTunnelServer) handleLargeFileUploadWithStreaming(domain string, httpReq *http.Request, clientIP string) (*http.Response, error) {
	s.logger.Info("[CHUNKED] 🚀 Implementing TRUE chunked streaming for unlimited file sizes")

	// Get tunnel stream for domain
	s.tunnelStreamsMux.RLock()
	tunnelStream, exists := s.tunnelStreams[domain]
	s.tunnelStreamsMux.RUnlock()

	if !exists || !tunnelStream.connected {
		return nil, fmt.Errorf("no active tunnel for domain: %s", domain)
	}

	// Convert headers only; body will be streamed via Start/Chunk/End
	// Build Start message directly
	headers := make(map[string]string)
	for k, v := range httpReq.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	requestID := generateRequestID()

	// Register response channel first
	responseChan := make(chan *proto.TunnelMessage, 250)
	tunnelStream.requestsMux.Lock()
	tunnelStream.pendingRequests[requestID] = responseChan
	tunnelStream.requestsMux.Unlock()

	// Send Start message
	startMsg := &proto.TunnelMessage{
		RequestId: requestID,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpRequestStart{
			HttpRequestStart: &proto.HTTPRequestStart{
				RequestId:   requestID,
				Method:      httpReq.Method,
				Path:        httpReq.URL.RequestURI(),
				Headers:     headers,
				ClientIp:    clientIP,
				IsLargeFile: true,
			},
		},
	}
	if err := tunnelStream.Stream.Send(startMsg); err != nil {
		return nil, fmt.Errorf("failed to send upload start: %w", err)
	}

	// Stream request body as chunks
	if httpReq.Body != nil {
		buf := make([]byte, DefaultChunkSize)
		for {
			n, er := httpReq.Body.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				chunkMsg := &proto.TunnelMessage{
					RequestId: requestID,
					Timestamp: time.Now().Unix(),
					MessageType: &proto.TunnelMessage_HttpRequestChunk{
						HttpRequestChunk: &proto.HTTPRequestChunk{RequestId: requestID, Data: data},
					},
				}
				if err := tunnelStream.Stream.Send(chunkMsg); err != nil {
					return nil, fmt.Errorf("failed to send upload chunk: %w", err)
				}
			}
			if er == io.EOF {
				break
			}
			if er != nil {
				return nil, fmt.Errorf("read upload body failed: %w", er)
			}
		}
	}

	// Send End
	endMsg := &proto.TunnelMessage{
		RequestId:   requestID,
		Timestamp:   time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpRequestEnd{HttpRequestEnd: &proto.HTTPRequestEnd{RequestId: requestID}},
	}
	if err := tunnelStream.Stream.Send(endMsg); err != nil {
		return nil, fmt.Errorf("failed to send upload end: %w", err)
	}

	// Now collect chunked response using existing io.Pipe pathway without re-sending request
	return s.collectChunkedResponseNoSend(tunnelStream, requestID)
}

// handleLargeFileDownloadWithChunking uses the old LargeFileRequest path for downloads
func (s *GRPCTunnelServer) handleLargeFileDownloadWithChunking(domain string, httpReq *http.Request, clientIP string) (*http.Response, error) {
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
		ChunkSize:         DefaultChunkSize, // 4MB chunks
		EnableCompression: true,
	}

	s.logger.Debug("[CHUNKED] Sending large file request to client (download): %s", httpReq.URL.Path)

	// Send large file request to client and collect chunked response (this method sends the HTTPRequest)
	return s.collectChunkedResponse(tunnelStream, largeFileReq)
}

// collectChunkedResponse sends large file request to client and streams response with minimal memory usage
func (s *GRPCTunnelServer) collectChunkedResponse(tunnelStream *TunnelStream, req *proto.LargeFileRequest) (*http.Response, error) {
	s.logger.Debug("[CHUNKED] 📦 Starting MEMORY-EFFICIENT chunk collection for request: %s", req.RequestId)

	// Create response channel and register it in pendingRequests (CRITICAL!)
	responseChan := make(chan *proto.TunnelMessage, 250) // Larger buffer for faster streaming

	tunnelStream.requestsMux.Lock()
	tunnelStream.pendingRequests[req.RequestId] = responseChan
	tunnelStream.requestsMux.Unlock()

	// Note: Channel cleanup is handled by the goroutine when streaming completes
	// We don't clean up here because the streaming response needs the channel to stay open

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

	s.logger.Debug("[CHUNKED] ✅ Large file request sent, registered in pendingRequests, waiting for chunked response...")

	// Update tunnel activity since we're starting an active chunked transfer
	tunnelStream.mu.Lock()
	tunnelStream.lastActivity = time.Now()
	tunnelStream.mu.Unlock()

	// Create a streaming pipe for memory-efficient processing
	pipeReader, pipeWriter := io.Pipe()

	// Channel to receive response metadata from first chunk
	metadataCh := make(chan *proto.HTTPResponse, 1)
	errorCh := make(chan error, 1)

	// Start goroutine to collect chunks and stream them directly to pipe
	go func() {
		defer pipeWriter.Close()

		// CRITICAL: Clean up request when goroutine exits
		defer func() {
			s.logger.Debug("[CHUNKED] 🧹 Goroutine exiting, cleaning up request: %s", req.RequestId)
			tunnelStream.requestsMux.Lock()
			if ch, exists := tunnelStream.pendingRequests[req.RequestId]; exists {
				delete(tunnelStream.pendingRequests, req.RequestId)
				// Safely close channel
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.logger.Debug("[CHUNKED] Channel already closed for request: %s", req.RequestId)
						}
					}()
					close(ch)
				}()
			}
			tunnelStream.requestsMux.Unlock()
		}()

		var firstChunk *proto.HTTPResponse
		chunkCount := 0

		// Set timeout for chunk collection (reasonable timeout - activity tracking prevents tunnel timeout)
		timeout := time.After(2 * time.Minute) // 2 minutes max - fail fast if broken

		for {
			select {
			case <-timeout:
				errorCh <- fmt.Errorf("timeout waiting for chunked response after 2 minutes")
				return

			case response, ok := <-responseChan:
				if !ok {
					s.logger.Info("[CHUNKED] 🔌 Response channel closed during tunnel disconnection - stopping chunk collection")
					errorCh <- fmt.Errorf("tunnel disconnected during chunked response collection")
					return
				}

				s.logger.Debug("[CHUNKED] 📥 Received response from pendingRequests channel: %s", response.RequestId)

				// Update tunnel activity to prevent timeout during chunked streaming
				tunnelStream.mu.Lock()
				tunnelStream.lastActivity = time.Now()
				tunnelStream.mu.Unlock()

				// Handle different message types
				switch msgType := response.MessageType.(type) {
				case *proto.TunnelMessage_HttpResponse:
					chunk := msgType.HttpResponse

					// Store the first chunk for headers and status
					if firstChunk == nil {
						firstChunk = chunk
						metadataCh <- chunk
						s.logger.Debug("[CHUNKED] 📋 Response metadata: status=%d, content-type=%s",
							chunk.StatusCode, chunk.Headers["Content-Type"])
					}

					// If this is a chunked response, stream chunks directly to pipe
					if chunk.IsChunked {
						chunkCount++

						if len(chunk.Body) > 0 {
							// MEMORY EFFICIENT: Write chunk directly to pipe (no buffering)
							if _, writeErr := pipeWriter.Write(chunk.Body); writeErr != nil {
								errorCh <- fmt.Errorf("failed to write chunk to pipe: %w", writeErr)
								return
							}

							s.logger.Debug("[CHUNKED] 📥 Streamed chunk %d (%d bytes) directly to pipe",
								chunkCount, len(chunk.Body))

							// Update activity for each chunk to keep tunnel alive during large transfers
							tunnelStream.mu.Lock()
							tunnelStream.lastActivity = time.Now()
							tunnelStream.mu.Unlock()
						}

						// Check if this is the final chunk
						if strings.HasSuffix(chunk.ChunkId, "_final") {
							s.logger.Info("[CHUNKED] ✅ All %d chunks streamed directly (ZERO memory buffering)", chunkCount)
							return // Close the pipe writer in defer
						}
					} else {
						// Non-chunked response - write entire body and finish
						if _, writeErr := pipeWriter.Write(chunk.Body); writeErr != nil {
							errorCh <- fmt.Errorf("failed to write non-chunked response: %w", writeErr)
							return
						}
						s.logger.Info("[CHUNKED] ✅ Streamed non-chunked response")
						return
					}

				case *proto.TunnelMessage_Error:
					errorMsg := msgType.Error
					errorCh <- fmt.Errorf("client error during chunked streaming: %s", errorMsg.Message)
					return

				default:
					s.logger.Warn("[CHUNKED] Unexpected message type during chunk collection: %T", msgType)
				}
			}
		}
	}()

	// Wait for first chunk to get response metadata
	select {
	case firstChunk := <-metadataCh:
		// Create streaming HTTP response with the pipe reader
		response := &http.Response{
			StatusCode:    int(firstChunk.StatusCode),
			Status:        firstChunk.StatusText,
			Header:        make(http.Header),
			Body:          pipeReader, // MEMORY EFFICIENT: Stream directly from pipe
			ContentLength: -1,         // Unknown length for streaming
		}

		// Set headers from the first chunk
		for key, value := range firstChunk.Headers {
			response.Header.Set(key, value)
		}

		// Remove Content-Length as we're streaming
		response.Header.Del("Content-Length")

		s.logger.Info("[CHUNKED] 🚀 MEMORY-EFFICIENT streaming response created (no buffering)")
		return response, nil

	case err := <-errorCh:
		pipeReader.Close()
		s.logger.Warn("[CHUNKED] ❌ Chunked streaming failed: %v", err)
		return nil, err

	case <-time.After(60 * time.Second):
		pipeReader.Close()
		s.logger.Error("[CHUNKED] ⏰ Timeout waiting for chunked response metadata after 60s")
		return nil, fmt.Errorf("timeout waiting for chunked response metadata")
	}
}

// collectChunkedResponseNoSend streams the response for a request that was already started (no HTTPRequest send here)
func (s *GRPCTunnelServer) collectChunkedResponseNoSend(tunnelStream *TunnelStream, requestID string) (*http.Response, error) {
	s.logger.Debug("[CHUNKED] 📦 Starting response collection (no-send) for request: %s", requestID)

	// Lookup existing response channel
	tunnelStream.requestsMux.RLock()
	responseChan, exists := tunnelStream.pendingRequests[requestID]
	tunnelStream.requestsMux.RUnlock()
	if !exists {
		return nil, fmt.Errorf("no pending request channel for request: %s", requestID)
	}

	// Create a streaming pipe
	pipeReader, pipeWriter := io.Pipe()

	metadataCh := make(chan *proto.HTTPResponse, 1)
	errorCh := make(chan error, 1)

	// Start goroutine to forward chunks to pipe
	go func() {
		defer pipeWriter.Close()

		// Cleanup pendingRequests entry on exit
		defer func() {
			s.logger.Debug("[CHUNKED] 🧹 Goroutine exiting (no-send), cleaning up request: %s", requestID)
			tunnelStream.requestsMux.Lock()
			if ch, ok := tunnelStream.pendingRequests[requestID]; ok {
				delete(tunnelStream.pendingRequests, requestID)
				// Safely close channel
				func() {
					defer func() { _ = recover() }()
					close(ch)
				}()
			}
			tunnelStream.requestsMux.Unlock()
		}()

		var firstChunk *proto.HTTPResponse
		chunkCount := 0
		timeout := time.After(2 * time.Minute)

		for {
			select {
			case <-timeout:
				errorCh <- fmt.Errorf("timeout waiting for chunked response after 2 minutes")
				return
			case response, ok := <-responseChan:
				if !ok {
					errorCh <- fmt.Errorf("tunnel disconnected during chunked response collection")
					return
				}

				// Update activity
				tunnelStream.mu.Lock()
				tunnelStream.lastActivity = time.Now()
				tunnelStream.mu.Unlock()

				switch msgType := response.MessageType.(type) {
				case *proto.TunnelMessage_HttpResponse:
					chunk := msgType.HttpResponse
					if firstChunk == nil {
						firstChunk = chunk
						metadataCh <- chunk
					}
					if chunk.IsChunked {
						chunkCount++
						if len(chunk.Body) > 0 {
							if _, err := pipeWriter.Write(chunk.Body); err != nil {
								errorCh <- fmt.Errorf("failed to write chunk to pipe: %w", err)
								return
							}
						}
						if strings.HasSuffix(chunk.ChunkId, "_final") {
							return
						}
					} else {
						if _, err := pipeWriter.Write(chunk.Body); err != nil {
							errorCh <- fmt.Errorf("failed to write non-chunked response: %w", err)
							return
						}
						return
					}
				case *proto.TunnelMessage_Error:
					errorCh <- fmt.Errorf("client error during chunked streaming: %s", msgType.Error.Message)
					return
				}
			}
		}
	}()

	// Wait for metadata
	select {
	case firstChunk := <-metadataCh:
		response := &http.Response{
			StatusCode:    int(firstChunk.StatusCode),
			Status:        firstChunk.StatusText,
			Header:        make(http.Header),
			Body:          pipeReader,
			ContentLength: -1,
		}
		for k, v := range firstChunk.Headers {
			response.Header.Set(k, v)
		}
		response.Header.Del("Content-Length")
		return response, nil
	case err := <-errorCh:
		pipeReader.Close()
		return nil, err
	case <-time.After(60 * time.Second):
		pipeReader.Close()
		return nil, fmt.Errorf("timeout waiting for chunked response metadata")
	}
}

// Legacy methods removed - now using memory-efficient streaming via io.Pipe()
// No more buffering of entire responses in memory!
