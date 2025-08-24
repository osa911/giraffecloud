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

	// If client provided a domain, try to match it
	if handshake.Domain != "" {
		for _, t := range tunnels {
			if t.Domain == handshake.Domain {
				return t, nil
			}
		}
		return nil, fmt.Errorf("no tunnel found for domain: %s", handshake.Domain)
	}

	// Proper fix: allow empty domain – resolve by token
	// Prefer an active tunnel; fallback to the first available
	var candidate *ent.Tunnel
	for _, t := range tunnels {
		if t.IsActive {
			candidate = t
			break
		}
		if candidate == nil {
			candidate = t
		}
	}
	if candidate == nil {
		return nil, fmt.Errorf("no tunnels available for user")
	}
	s.logger.Info("[gRPC AUTH] Resolved tunnel by token: domain=%s, target_port=%d", candidate.Domain, candidate.TargetPort)
	return candidate, nil
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
		case *proto.TunnelMessage_HttpRequestStart:
			s.handleUploadStart(tunnelStream, msg)
		case *proto.TunnelMessage_HttpRequestChunk:
			s.handleUploadChunk(tunnelStream, msg)
		case *proto.TunnelMessage_HttpRequestEnd:
			s.handleUploadEnd(tunnelStream, msg)
		case *proto.TunnelMessage_Control:
			s.handleControlMessage(tunnelStream, msg)
		case *proto.TunnelMessage_Error:
			s.handleErrorMessage(tunnelStream, msg)
		default:
			s.logger.Warn("Unknown message type from tunnel %s: %T", tunnelStream.Domain, msgType)
		}
	}
}

// handleUploadStart initializes per-request channels and pipe for streaming upload
func (s *GRPCTunnelServer) handleUploadStart(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	start := msg.GetHttpRequestStart()
	if start == nil {
		return
	}
	// Create per-request response channel if not present
	tunnelStream.requestsMux.Lock()
	if _, exists := tunnelStream.pendingRequests[start.RequestId]; !exists {
		tunnelStream.pendingRequests[start.RequestId] = make(chan *proto.TunnelMessage, 64)
	}
	tunnelStream.requestsMux.Unlock()
}

// handleUploadChunk forwards chunk messages to the waiting request goroutine
func (s *GRPCTunnelServer) handleUploadChunk(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	tunnelStream.requestsMux.RLock()
	ch, ok := tunnelStream.pendingRequests[msg.RequestId]
	tunnelStream.requestsMux.RUnlock()
	if !ok {
		s.logger.Warn("Received upload chunk for unknown request ID: %s", msg.RequestId)
		return
	}
	select {
	case ch <- msg:
	default:
		s.logger.Warn("Backpressure: upload chunk channel full for request: %s", msg.RequestId)
	}
}

// handleUploadEnd forwards end marker
func (s *GRPCTunnelServer) handleUploadEnd(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	tunnelStream.requestsMux.RLock()
	ch, ok := tunnelStream.pendingRequests[msg.RequestId]
	tunnelStream.requestsMux.RUnlock()
	if !ok {
		s.logger.Warn("Received upload end for unknown request ID: %s", msg.RequestId)
		return
	}
	select {
	case ch <- msg:
	default:
		s.logger.Warn("Backpressure: upload end channel full for request: %s", msg.RequestId)
	}
}

// handleHTTPResponse handles HTTP response messages from the client
func (s *GRPCTunnelServer) handleHTTPResponse(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	httpResp := msg.GetHttpResponse()
	if httpResp == nil {
		s.logger.Warn("Received invalid HTTP response message")
		return
	}

	// Note: Chunked response handling is now done via memory-efficient streaming
	// in collectChunkedResponse() using io.Pipe(). The old buffering approach
	// has been removed to prevent conflicts and memory issues.

	// Handle all responses as regular responses (streaming is handled at request level)
	s.handleRegularHTTPResponse(tunnelStream, msg)
}

// handleRegularHTTPResponse handles regular (non-chunked) HTTP responses
func (s *GRPCTunnelServer) handleRegularHTTPResponse(tunnelStream *TunnelStream, msg *proto.TunnelMessage) {
	tunnelStream.requestsMux.RLock()
	responseChan, exists := tunnelStream.pendingRequests[msg.RequestId]
	tunnelStream.requestsMux.RUnlock()

	if !exists {
		// Suppress noisy warnings for late chunks after a client disconnect/cleanup
		if httpResp := msg.GetHttpResponse(); httpResp != nil && httpResp.IsChunked {
			s.logger.Debug("Late chunk for cleaned-up request ID: %s (dropping)", msg.RequestId)
			return
		}
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

	// Note: Chunked requests are cleaned up by their goroutines
	// Only clean up non-chunked requests here
	httpResponse := msg.GetHttpResponse()
	if httpResponse == nil || !httpResponse.IsChunked {
		// Clean up non-chunked requests immediately
		tunnelStream.requestsMux.Lock()
		delete(tunnelStream.pendingRequests, msg.RequestId)
		close(responseChan)
		tunnelStream.requestsMux.Unlock()
	}
}

// Legacy chunked response methods removed - now using memory-efficient streaming via io.Pipe()
// Old methods handleChunkedHTTPResponse, checkChunkedResponseCompletion, and
// assembleAndDeliverChunkedResponse have been replaced with collectChunkedResponse()
// in grpc_chunked_streaming.go which uses streaming instead of buffering.

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
			// Check if tunnel is still active (10 minutes - reasonable for photo gallery browsing)
			if time.Since(tunnelStream.lastActivity) > 10*time.Minute {
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
	// Preserve upstream headers
	for k, v := range httpResp.Headers {
		resp.Header.Set(k, v)
	}

	// Convert headers
	for key, value := range httpResp.Headers {
		resp.Header.Set(key, value)
	}

	// Record usage best-effort (response bytes). Requests counted at call site.
	if s.usage != nil {
		s.tunnelStreamsMux.RLock()
		// We don't know domain here; embed in message headers if needed. For now, try X-Forwarded-Host.
		domain := httpResp.Headers["X-Forwarded-Host"]
		if domain == "" {
			domain = httpResp.Headers["Host"]
		}
		// Find stream by domain to get user/tunnel IDs
		var userID uint32
		var tunnelID uint32
		if ts, ok := s.tunnelStreams[domain]; ok {
			userID = ts.UserID
			tunnelID = ts.TunnelID
		}
		s.tunnelStreamsMux.RUnlock()
		if domain != "" && len(httpResp.Body) > 0 && userID != 0 {
			s.usage.Increment(userID, tunnelID, domain, 0, int64(len(httpResp.Body)), 1)
		}
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
