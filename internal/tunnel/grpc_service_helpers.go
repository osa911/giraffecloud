package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc/peer"
)

// RateLimiter provides rate limiting functionality for tunnel requests
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rpm      int
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rpm, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rpm:      rpm,
		burst:    burst,
	}
}

// Allow checks if the request is allowed for the given domain
func (rl *RateLimiter) Allow(domain string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[domain]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rl.rpm)/60, rl.burst) // Convert RPM to per-second
		rl.limiters[domain] = limiter
	}

	return limiter.Allow()
}

// SecurityMiddleware provides security functionality
type SecurityMiddleware struct {
	allowedOrigins map[string]bool
	mu             sync.RWMutex
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware() *SecurityMiddleware {
	return &SecurityMiddleware{
		allowedOrigins: make(map[string]bool),
	}
}

// ValidateOrigin checks if the origin is allowed
func (sm *SecurityMiddleware) ValidateOrigin(origin string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.allowedOrigins[origin]
}

// getPeerIP extracts the client IP from the gRPC context
func getPeerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	// Extract IP from the address
	if addr, ok := p.Addr.(*net.TCPAddr); ok {
		return addr.IP.String()
	}

	// Fallback to string representation
	return p.Addr.String()
}

// authenticateTunnel authenticates a tunnel handshake
func (s *GRPCTunnelServer) authenticateTunnel(ctx context.Context, handshake *proto.TunnelHandshake) (*ent.Tunnel, error) {
	// Find user by API token
	token, err := s.tokenRepo.GetByToken(ctx, handshake.Token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get user's tunnels
	tunnels, err := s.tunnelRepo.GetByUserID(ctx, token.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnels: %w", err)
	}

	// Find matching domain
	for _, tunnel := range tunnels {
		if tunnel.Domain == handshake.Domain {
			return tunnel, nil
		}
	}

	return nil, fmt.Errorf("no tunnel found for domain: %s", handshake.Domain)
}

// handleClientMessages handles incoming messages from the client tunnel
func (s *GRPCTunnelServer) handleClientMessages(tunnelStream *TunnelStream) {
	for {
		msg, err := tunnelStream.Stream.Recv()
		if err != nil {
			s.logger.Error("Error receiving message from tunnel %s: %v", tunnelStream.Domain, err)
			tunnelStream.connected = false
			return
		}

		tunnelStream.lastActivity = time.Now()

		// Handle different message types
		switch msgType := msg.MessageType.(type) {
		case *proto.TunnelMessage_HttpResponse:
			s.handleHTTPResponse(tunnelStream, msg)
		case *proto.TunnelMessage_Control:
			s.handleControlMessage(tunnelStream, msg)
		case *proto.TunnelMessage_Error:
			s.handleErrorMessage(tunnelStream, msg)
		default:
			s.logger.Warn("Unknown message type from tunnel %s: %T", tunnelStream.Domain, msgType)
		}
	}
}

// handleHTTPResponse handles HTTP response messages from the client
func (s *GRPCTunnelServer) handleHTTPResponse(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	httpResp := msg.GetHttpResponse()
	if httpResp == nil {
		s.logger.Warn("Received invalid HTTP response message")
		return
	}

	// Check if this is a chunked response for unlimited file size support
	if httpResp.IsChunked {
		s.handleChunkedHTTPResponse(tunnelStream, msg, httpResp)
		return
	}

	// Handle regular (non-chunked) responses
	s.handleRegularHTTPResponse(tunnelStream, msg)
}

// handleRegularHTTPResponse handles regular (non-chunked) HTTP responses
func (s *GRPCTunnelServer) handleRegularHTTPResponse(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	tunnelStream.requestsMux.RLock()
	responseChan, exists := tunnelStream.pendingRequests[msg.RequestId]
	tunnelStream.requestsMux.RUnlock()

	if !exists {
		s.logger.Warn("Received response for unknown request ID: %s", msg.RequestId)
		return
	}

	// Send response to waiting goroutine
	select {
	case responseChan <- msg:
		// Response delivered
	case <-time.After(5 * time.Second):
		s.logger.Warn("Timeout delivering response for request ID: %s", msg.RequestId)
	}

	// Clean up pending request
	tunnelStream.requestsMux.Lock()
	delete(tunnelStream.pendingRequests, msg.RequestId)
	close(responseChan)
	tunnelStream.requestsMux.Unlock()
}

// handleChunkedHTTPResponse handles chunked HTTP responses for unlimited file sizes
func (s *GRPCTunnelServer) handleChunkedHTTPResponse(tunnelStream *TunnelStream, msg *proto.TunnelMessage, httpResp *proto.HTTPResponse) {
	requestID := msg.RequestId
	chunkID := httpResp.ChunkId

	s.logger.Debug("[CHUNKED SERVER] 📦 Received chunk: %s for request: %s", chunkID, requestID)

	// Get or create chunk buffer for this request
	tunnelStream.requestsMux.Lock()
	if tunnelStream.chunkedResponses == nil {
		tunnelStream.chunkedResponses = make(map[string]*ChunkedResponseBuffer)
	}

	buffer, exists := tunnelStream.chunkedResponses[requestID]
	if !exists {
		buffer = &ChunkedResponseBuffer{
			RequestID:    requestID,
			StatusCode:   httpResp.StatusCode,
			StatusText:   httpResp.StatusText,
			Headers:      httpResp.Headers,
			Chunks:       make([][]byte, 0),
			TotalSize:    0,
			StartTime:    time.Now(),
		}
		tunnelStream.chunkedResponses[requestID] = buffer
		s.logger.Debug("[CHUNKED SERVER] 🆕 Created new chunk buffer for request: %s", requestID)
	}
	tunnelStream.requestsMux.Unlock()

	// Add chunk to buffer
	buffer.mu.Lock()
	buffer.Chunks = append(buffer.Chunks, httpResp.Body)
	buffer.TotalSize += int64(len(httpResp.Body))
	buffer.LastChunkTime = time.Now()
	buffer.mu.Unlock()

	s.logger.Debug("[CHUNKED SERVER] ✅ Added chunk %s (%d bytes), total: %d bytes",
		chunkID, len(httpResp.Body), buffer.TotalSize)

	// Check if we should assemble and deliver the response
	// For now, we'll use a simple timeout-based approach
	// In production, you'd want a more sophisticated completion detection
	go s.checkChunkedResponseCompletion(tunnelStream, requestID, buffer)
}



// checkChunkedResponseCompletion checks if a chunked response is complete
func (s *GRPCTunnelServer) checkChunkedResponseCompletion(tunnelStream *TunnelStream, requestID string, buffer *ChunkedResponseBuffer) {
	// Wait a bit to see if more chunks arrive
	time.Sleep(100 * time.Millisecond)

	buffer.mu.Lock()
	timeSinceLastChunk := time.Since(buffer.LastChunkTime)
	buffer.mu.Unlock()

	// If no new chunks for 100ms, consider it complete
	if timeSinceLastChunk >= 100*time.Millisecond {
		s.assembleAndDeliverChunkedResponse(tunnelStream, requestID, buffer)
	}
}

// assembleAndDeliverChunkedResponse assembles chunks into final response
func (s *GRPCTunnelServer) assembleAndDeliverChunkedResponse(tunnelStream *TunnelStream, requestID string, buffer *ChunkedResponseBuffer) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	s.logger.Info("[CHUNKED SERVER] 🔧 Assembling %d chunks (%d bytes) for unlimited file response",
		len(buffer.Chunks), buffer.TotalSize)

	// Assemble all chunks into one response body
	totalBody := make([]byte, 0, buffer.TotalSize)
	for i, chunk := range buffer.Chunks {
		totalBody = append(totalBody, chunk...)
		s.logger.Debug("[CHUNKED SERVER] 📎 Assembled chunk %d (%d bytes)", i+1, len(chunk))
	}

	// Create final response message
	finalResponse := &proto.TunnelMessage{
		RequestId: requestID,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpResponse{
			HttpResponse: &proto.HTTPResponse{
				StatusCode: buffer.StatusCode,
				StatusText: buffer.StatusText,
				Headers:    buffer.Headers,
				Body:       totalBody,
				IsChunked:  false, // Final assembled response
			},
		},
	}

	s.logger.Info("[CHUNKED SERVER] 🎉 Delivering assembled response: %d bytes (UNLIMITED SIZE ACHIEVED!)", len(totalBody))

	// Deliver to waiting goroutine
	tunnelStream.requestsMux.RLock()
	responseChan, exists := tunnelStream.pendingRequests[requestID]
	tunnelStream.requestsMux.RUnlock()

	if exists {
		select {
		case responseChan <- finalResponse:
			s.logger.Debug("[CHUNKED SERVER] ✅ Successfully delivered unlimited size response")
		case <-time.After(5 * time.Second):
			s.logger.Warn("[CHUNKED SERVER] ⚠️ Timeout delivering chunked response")
		}

		// Clean up
		tunnelStream.requestsMux.Lock()
		delete(tunnelStream.pendingRequests, requestID)
		delete(tunnelStream.chunkedResponses, requestID)
		close(responseChan)
		tunnelStream.requestsMux.Unlock()
	}
}

// handleControlMessage handles control messages from the client
func (s *GRPCTunnelServer) handleControlMessage(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	control := msg.GetControl()
	if control == nil {
		return
	}

	switch controlType := control.ControlType.(type) {
	case *proto.TunnelControl_Status:
		status := controlType.Status
		s.logger.Debug("Tunnel %s status: %s, active connections: %d",
			tunnelStream.Domain, status.State, status.ActiveConnections)
	case *proto.TunnelControl_Metrics:
		metrics := controlType.Metrics
		s.logger.Debug("Tunnel %s metrics: %d requests, %.2f avg response time",
			tunnelStream.Domain, metrics.TotalRequests, metrics.AverageResponseTime)
	default:
		s.logger.Debug("Unknown control message type: %T", controlType)
	}
}

// handleErrorMessage handles error messages from the client
func (s *GRPCTunnelServer) handleErrorMessage(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	errorMsg := msg.GetError()
	if errorMsg == nil {
		return
	}

	s.logger.Error("Tunnel %s error: %s (code: %d)",
		tunnelStream.Domain, errorMsg.Message, errorMsg.Code)

	// If the error is for a specific request, handle it
	if msg.RequestId != "" {
		tunnelStream.requestsMux.RLock()
		responseChan, exists := tunnelStream.pendingRequests[msg.RequestId]
		tunnelStream.requestsMux.RUnlock()

		if exists {
			select {
			case responseChan <- msg:
				// Error delivered
			case <-time.After(1 * time.Second):
				s.logger.Warn("Timeout delivering error for request ID: %s", msg.RequestId)
			}

			// Clean up pending request
			tunnelStream.requestsMux.Lock()
			delete(tunnelStream.pendingRequests, msg.RequestId)
			close(responseChan)
			tunnelStream.requestsMux.Unlock()
		}
	}
}

// monitorTunnelHealth monitors the health of a tunnel stream
func (s *GRPCTunnelServer) monitorTunnelHealth(tunnelStream *TunnelStream) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tunnelStream.Context.Done():
			s.logger.Info("Tunnel context cancelled for domain: %s", tunnelStream.Domain)
			return tunnelStream.Context.Err()

		case <-ticker.C:
			// Check if tunnel is still active
			if time.Since(tunnelStream.lastActivity) > 2*time.Minute {
				s.logger.Warn("Tunnel %s inactive for %v, closing",
					tunnelStream.Domain, time.Since(tunnelStream.lastActivity))
				return fmt.Errorf("tunnel inactive")
			}

			// Send health check
			healthCheck := &proto.TunnelMessage{
				RequestId: fmt.Sprintf("health-%d", time.Now().Unix()),
				Timestamp: time.Now().Unix(),
				MessageType: &proto.TunnelMessage_Control{
					Control: &proto.TunnelControl{
						ControlType: &proto.TunnelControl_Status{
							Status: &proto.TunnelStatus{
								State:        proto.TunnelState_TUNNEL_STATE_CONNECTED,
								LastActivity: time.Now().Unix(),
							},
						},
					},
				},
			}

			if err := tunnelStream.Stream.Send(healthCheck); err != nil {
				s.logger.Error("Failed to send health check to tunnel %s: %v", tunnelStream.Domain, err)
				return err
			}
		}
	}
}

// reportMetrics reports server metrics periodically
func (s *GRPCTunnelServer) reportMetrics() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.tunnelStreamsMux.RLock()
		activeStreams := len(s.tunnelStreams)
		s.tunnelStreamsMux.RUnlock()

		totalReqs := atomic.LoadInt64(&s.totalRequests)
		concurrentReqs := atomic.LoadInt64(&s.concurrentReqs)
		totalResponses := atomic.LoadInt64(&s.totalResponses)
		totalErrors := atomic.LoadInt64(&s.totalErrors)

		s.logger.Info("[gRPC METRICS] Active Streams: %d, Total Requests: %d, Concurrent: %d, Responses: %d, Errors: %d",
			activeStreams, totalReqs, concurrentReqs, totalResponses, totalErrors)
	}
}

// httpToGRPC converts an HTTP request to a gRPC tunnel message
func (s *GRPCTunnelServer) httpToGRPC(req *http.Request, clientIP string) (*proto.TunnelMessage, error) {
	// Read request body
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
		// Restore body for potential reuse
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Convert headers
	headers := make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0] // Take first value for simplicity
		}
	}

	// Determine request type
	reqType := proto.RequestType_REQUEST_TYPE_API
	if strings.Contains(req.URL.Path, "/api/assets/") ||
	   strings.Contains(req.URL.Path, "/media/") ||
	   strings.Contains(req.URL.Path, "/static/") {
		reqType = proto.RequestType_REQUEST_TYPE_MEDIA
	}

	// Create gRPC message
	grpcMsg := &proto.TunnelMessage{
		RequestId: generateRequestID(),
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpRequest{
			HttpRequest: &proto.HTTPRequest{
				Method:   req.Method,
				Path:     req.URL.RequestURI(),
				Headers:  headers,
				Body:     body,
				ClientIp: clientIP,
				Metadata: &proto.RequestMetadata{
					Type:     reqType,
					Priority: proto.Priority_PRIORITY_NORMAL,
				},
			},
		},
	}

	return grpcMsg, nil
}

// sendRequestAndWaitResponse sends a request and waits for the response
func (s *GRPCTunnelServer) sendRequestAndWaitResponse(tunnelStream *TunnelStream, grpcMsg *proto.TunnelMessage) (*http.Response, error) {
	// Create response channel
	responseChan := make(chan *proto.TunnelMessage, 1)

	// Register pending request
	tunnelStream.requestsMux.Lock()
	tunnelStream.pendingRequests[grpcMsg.RequestId] = responseChan
	tunnelStream.requestsMux.Unlock()

	// Send request to client
	if err := tunnelStream.Stream.Send(grpcMsg); err != nil {
		// Clean up on send failure
		tunnelStream.requestsMux.Lock()
		delete(tunnelStream.pendingRequests, grpcMsg.RequestId)
		tunnelStream.requestsMux.Unlock()
		close(responseChan)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response with timeout
	timeout := s.config.RequestTimeout
	select {
	case responseMsg := <-responseChan:
		// Convert response back to HTTP
		return s.grpcToHTTP(responseMsg)

	case <-time.After(timeout):
		// Clean up on timeout
		tunnelStream.requestsMux.Lock()
		delete(tunnelStream.pendingRequests, grpcMsg.RequestId)
		tunnelStream.requestsMux.Unlock()
		close(responseChan)
		return nil, fmt.Errorf("request timeout after %v", timeout)

	case <-tunnelStream.Context.Done():
		// Clean up on context cancellation
		tunnelStream.requestsMux.Lock()
		delete(tunnelStream.pendingRequests, grpcMsg.RequestId)
		tunnelStream.requestsMux.Unlock()
		close(responseChan)
		return nil, fmt.Errorf("tunnel disconnected")
	}
}

// grpcToHTTP converts a gRPC tunnel message back to an HTTP response
func (s *GRPCTunnelServer) grpcToHTTP(msg *proto.TunnelMessage) (*http.Response, error) {
	// Handle error messages
	if errorMsg := msg.GetError(); errorMsg != nil {
		return nil, fmt.Errorf("tunnel error: %s (code: %d)", errorMsg.Message, errorMsg.Code)
	}

	// Get HTTP response
	httpResp := msg.GetHttpResponse()
	if httpResp == nil {
		return nil, fmt.Errorf("invalid response message")
	}

	// Create HTTP response
	resp := &http.Response{
		StatusCode: int(httpResp.StatusCode),
		Status:     fmt.Sprintf("%d %s", httpResp.StatusCode, httpResp.StatusText),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(httpResp.Body)),
	}

	// Convert headers
	for key, value := range httpResp.Headers {
		resp.Header.Set(key, value)
	}

	return resp, nil
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// IsTunnelActive checks if a tunnel is active for the given domain
func (s *GRPCTunnelServer) IsTunnelActive(domain string) bool {
	s.tunnelStreamsMux.RLock()
	defer s.tunnelStreamsMux.RUnlock()

	stream, exists := s.tunnelStreams[domain]
	return exists && stream.connected
}