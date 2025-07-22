package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

/**
	FYI: We use this server to handle the handshake and proxy the connection to the local service

TODO:
- Add rate limiting
- Add logging
- Add metrics
*/

// ClientIPUpdateFunc is a callback function for client IP updates
type ClientIPUpdateFunc func(ctx context.Context, tunnelID uint32, clientIP string) error

// TunnelServer represents the tunnel server
type TunnelServer struct {
	listener      net.Listener
	logger        *logging.Logger
	tlsConfig     *tls.Config
	tokenRepo     repository.TokenRepository
	tunnelRepo    repository.TunnelRepository
	tunnelService interfaces.TunnelService
	connections   *ConnectionManager
	streamConfig  *StreamingConfig // Streaming configuration

	// Performance monitoring
	requestCount    int64    // Total requests handled
	concurrentReqs  int64    // Current concurrent requests
	poolHits        int64    // Successful pool connections
	poolMisses      int64    // Failed pool connections

	// Connection health monitoring
	lastCleanup     time.Time // Last cleanup time
	cleanupStats    map[string]int // Cleanup statistics
}

// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository, tunnelService interfaces.TunnelService) *TunnelServer {
	return &TunnelServer{
		logger:       logging.GetGlobalLogger(),
		connections:  NewConnectionManager(),
		streamConfig: DefaultStreamingConfig(), // Use default streaming config
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true, // Simplified for development
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
		},
		tokenRepo:     tokenRepo,
		tunnelRepo:    tunnelRepo,
		tunnelService: tunnelService,
	}
}

// Start starts the tunnel server
func (s *TunnelServer) Start(addr string) error {
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.listener = tls.NewListener(tcpListener, s.tlsConfig)
	go s.acceptConnections()

	s.logger.Info("Tunnel server listening on %s", addr)
	return nil
}

// Stop stops the tunnel server
func (s *TunnelServer) Stop() error {
	if s.listener == nil {
		return nil
	}

	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	return nil
}



// UpdateStreamingConfig updates the streaming configuration
func (s *TunnelServer) UpdateStreamingConfig(config *StreamingConfig) {
	s.streamConfig = config
	s.logger.Info("Updated streaming configuration: MediaOptimization=%v, PoolSize=%d, MediaBufferSize=%d",
		config.EnableMediaOptimization, config.PoolSize, config.MediaBufferSize)
}

// GetStreamingConfig returns the current streaming configuration
func (s *TunnelServer) GetStreamingConfig() *StreamingConfig {
	return s.streamConfig
}

func (s *TunnelServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Error("Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles a new tunnel connection
func (s *TunnelServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Create JSON encoder/decoder
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Read handshake request
	var req TunnelHandshakeRequest
	if err := decoder.Decode(&req); err != nil {
		s.logger.Error("Failed to decode handshake: %v", err)
		return
	}

	// Find user by API token
	token, err := s.tokenRepo.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to authenticate: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Invalid token. Please login first.",
		})
		return
	}

	// Get user's first tunnel
	tunnels, err := s.tunnelRepo.GetByUserID(context.Background(), token.UserID)
	if err != nil || len(tunnels) == 0 {
		s.logger.Error("No tunnels found for user: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "No tunnels found. Please create a tunnel first.",
		})
		return
	}

	tunnel := tunnels[0] // Use the first tunnel

	s.logger.Info("User %d connected with token %s", tunnel.UserID, tunnel.Token)

	// Get client IP from connection
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		s.logger.Error("Failed to get client IP: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to get client IP",
		})
		return
	}

	// Update client IP using tunnel service
	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to update client IP",
		})
		return
	}

	// Determine connection type based on request
	connType := ConnectionTypeHTTP
	if req.ConnectionType == "websocket" {
		connType = ConnectionTypeWebSocket
	}

	// Send success response with domain and port
	if err := encoder.Encode(TunnelHandshakeResponse{
		Status:     "success",
		Message:    "Connected successfully",
		Domain:     tunnel.Domain,
		TargetPort: tunnel.TargetPort,
		ConnectionType: string(connType),
	}); err != nil {
		s.logger.Error("Failed to send response: %v", err)
		return
	}

	// Create connection object and add to manager with type
	s.connections.AddConnection(tunnel.Domain, conn, tunnel.TargetPort, connType)
	defer s.connections.RemoveConnection(tunnel.Domain, connType)

	s.logger.Info("Tunnel connection established for domain: %s (type: %s)", tunnel.Domain, connType)

	// Keep the connection alive without interfering with HTTP traffic
	// The connection will be closed when the client disconnects or an error occurs
	// ProxyConnection will handle all HTTP communication
	select {}
}

// GetConnection returns the tunnel connection for a domain (backward compatibility for HTTP)
func (s *TunnelServer) GetConnection(domain string) *TunnelConnection {
	return s.connections.GetHTTPConnection(domain)
}

// IsTunnelDomain returns true if the domain has an active tunnel
func (s *TunnelServer) IsTunnelDomain(domain string) bool {
	return s.connections.HasDomain(domain)
}

// HasWebSocketConnection returns true if the domain has an active WebSocket tunnel
func (s *TunnelServer) HasWebSocketConnection(domain string) bool {
	return s.connections.HasWebSocketConnection(domain)
}

// ProxyConnection handles proxying an HTTP connection to the appropriate tunnel
func (s *TunnelServer) ProxyConnection(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	defer conn.Close()

	// Performance monitoring
	atomic.AddInt64(&s.requestCount, 1)
	concurrent := atomic.AddInt64(&s.concurrentReqs, 1)
	defer atomic.AddInt64(&s.concurrentReqs, -1)

		// Log performance metrics every 50 requests and perform cleanup
	if atomic.LoadInt64(&s.requestCount)%50 == 0 {
		poolSize := s.connections.GetHTTPPoolSize(domain)
		hits := atomic.LoadInt64(&s.poolHits)
		misses := atomic.LoadInt64(&s.poolMisses)

		// Perform periodic cleanup every 1 minute (more aggressive)
		now := time.Now()
		if now.Sub(s.lastCleanup) > 1*time.Minute {
			cleanupStats := s.connections.CleanupDeadConnections()
			if len(cleanupStats) > 0 {
				s.logger.Info("[CLEANUP] Removed dead connections: %v", cleanupStats)
			}

			// Also proactively recycle old connections
			s.recycleOldConnections(domain)
			s.lastCleanup = now
		}

		s.logger.Info("[PERF] Requests: %d, Concurrent: %d, Pool Size: %d, Hits: %d, Misses: %d",
			atomic.LoadInt64(&s.requestCount), concurrent, poolSize, hits, misses)
	}

	tunnelConn := s.connections.GetHTTPConnection(domain)
	if tunnelConn == nil {
		atomic.AddInt64(&s.poolMisses, 1)
		s.logger.Error("No HTTP tunnel connection found for domain: %s", domain)
		s.writeHTTPError(conn, 502, "Bad Gateway - HTTP tunnel not connected")
		return
	}

	// Successfully got a connection from the pool
	atomic.AddInt64(&s.poolHits, 1)

	// Increment request count for this specific connection
	tunnelConn.IncrementRequestCount()

	// Validate connection is still alive before using it
	if tunnelConn.GetConn() == nil {
		s.logger.Error("HTTP tunnel connection is closed for domain: %s", domain)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		s.writeHTTPError(conn, 502, "Bad Gateway - HTTP tunnel connection closed")
		return
	}

	// Set a short deadline for connection validation
	tunnelConn.GetConn().SetDeadline(time.Now().Add(1 * time.Second))
	// Test the connection with a small write (will fail immediately if connection is dead)
	if _, err := tunnelConn.GetConn().Write([]byte{}); err != nil {
		s.logger.Error("HTTP tunnel connection failed validation for domain: %s, error: %v", domain, err)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		// Try to get another connection immediately
		if retryConn := s.connections.GetHTTPConnection(domain); retryConn != nil && retryConn != tunnelConn {
			s.logger.Info("Switching to backup connection after validation failure")
			tunnelConn = retryConn
		} else {
			s.writeHTTPError(conn, 502, "Bad Gateway - No healthy connections available")
			return
		}
	}
	// Clear the validation deadline
	tunnelConn.GetConn().SetDeadline(time.Time{})

	// Check if this is a media/video request that should use optimized handling
	isMediaRequest := s.isMediaRequest(requestData)

	if isMediaRequest {
		// For media requests, use optimized handling
		s.proxyMediaRequest(domain, conn, requestData, requestBody)
		return
	}

	// CRITICAL: Lock this specific connection for HTTP/1.1 request-response cycle
	// This ensures proper sequencing while pool provides concurrency across connections
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	// Double-check connection after acquiring lock
	if tunnelConn.GetConn() == nil {
		s.logger.Error("HTTP tunnel connection closed after lock acquisition for domain: %s", domain)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		s.writeHTTPError(conn, 502, "Bad Gateway - HTTP tunnel connection closed")
		return
	}

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[PROXY DEBUG] Error writing request headers to tunnel: %v", err)
		// Connection is dead, remove it from pool
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)

		// Check if we should retry immediately for connection errors
		if s.isRetryableConnectionError(err) && s.connections.GetHTTPPoolSize(domain) > 1 {
			s.logger.Info("[PROXY DEBUG] Connection error detected, attempting immediate retry")
			s.retryWithFreshConnection(domain, conn, requestData, requestBody)
			return
		}

		s.writeHTTPError(conn, 502, "Bad Gateway - Failed to write to tunnel")
		return
	}

	// Copy request body if present
	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[PROXY DEBUG] Error writing request body to tunnel: %v", err)
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)

			// Check if we should retry immediately for connection errors
			if s.isRetryableConnectionError(err) && s.connections.GetHTTPPoolSize(domain) > 1 {
				s.logger.Info("[PROXY DEBUG] Body write error detected, attempting immediate retry")
				s.retryWithFreshConnection(domain, conn, requestData, requestBody)
				return
			}

			s.writeHTTPError(conn, 502, "Bad Gateway - Failed to write body to tunnel")
			return
		}
	}

	// Set a read timeout for regular requests
	regularTimeout := s.streamConfig.RegularTimeout
	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(regularTimeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{}) // Clear timeout

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())

			// Parse the response with better error handling
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading response from tunnel: %v", err)
		// Connection is corrupted, remove it from pool
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)

		// Try to get a fresh connection for retry (prioritize retryable errors)
		if s.isRetryableConnectionError(err) && s.connections.GetHTTPPoolSize(domain) > 1 {
			s.logger.Info("[PROXY DEBUG] Retryable error detected, attempting retry with fresh connection")
			// Note: Let the defer handle the unlock - don't unlock manually here
			s.retryWithFreshConnection(domain, conn, requestData, requestBody)
			return
		}

		s.writeHTTPError(conn, 502, "Bad Gateway - Tunnel response error")
		return
	}

	// Write the response back to the client
	clientWriter := bufio.NewWriter(conn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Debug("[PROXY DEBUG] Error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Debug("[PROXY DEBUG] Error flushing response: %v", err)
		return
	}

	// CRITICAL: Check if we should recycle this connection
	if s.shouldRecycleConnection(tunnelConn, response) {
		s.logger.Debug("[PROXY DEBUG] Recycling tunnel connection after request completion")
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		go tunnelConn.Close() // Close asynchronously to avoid blocking
	}
}

// retryWithFreshConnection attempts to retry the request with a fresh connection from the pool
func (s *TunnelServer) retryWithFreshConnection(domain string, clientConn net.Conn, requestData []byte, requestBody io.Reader) {
	retryTunnelConn := s.connections.GetHTTPConnection(domain)
	if retryTunnelConn == nil || retryTunnelConn.GetConn() == nil {
		s.logger.Error("[PROXY DEBUG] No fresh connection available for retry")
		s.writeHTTPError(clientConn, 502, "Bad Gateway - No connections available")
		return
	}

	s.logger.Info("[PROXY DEBUG] Retrying request with fresh connection from pool")

	// Lock the fresh connection
	retryTunnelConn.Lock()
	defer retryTunnelConn.Unlock()

	// Write the HTTP request headers to the fresh tunnel connection
	if _, err := retryTunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[PROXY DEBUG] Retry failed - error writing request headers: %v", err)
		s.connections.RemoveSpecificHTTPConnection(domain, retryTunnelConn)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Retry failed")
		return
	}

	// Copy request body if present (note: this might be empty if already consumed)
	if requestBody != nil {
		if _, err := io.Copy(retryTunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[PROXY DEBUG] Retry failed - error writing request body: %v", err)
			s.connections.RemoveSpecificHTTPConnection(domain, retryTunnelConn)
			s.writeHTTPError(clientConn, 502, "Bad Gateway - Retry failed")
			return
		}
	}

	// Set a read timeout for retry
	regularTimeout := s.streamConfig.RegularTimeout
	retryTunnelConn.GetConn().SetReadDeadline(time.Now().Add(regularTimeout))
	defer retryTunnelConn.GetConn().SetReadDeadline(time.Time{})

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(retryTunnelConn.GetConn())

	// Parse the response
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Retry failed - error reading response: %v", err)
		s.connections.RemoveSpecificHTTPConnection(domain, retryTunnelConn)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Retry failed")
		return
	}

	s.logger.Info("[PROXY DEBUG] Retry successful - received response: %s", response.Status)

	// Write the response back to the client
	clientWriter := bufio.NewWriter(clientConn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Debug("[PROXY DEBUG] Retry - error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Debug("[PROXY DEBUG] Retry - error flushing response: %v", err)
		return
	}

	s.logger.Info("[PROXY DEBUG] Retry completed successfully")
}

// isMediaRequest checks if the request is for media content that should be streamed
func (s *TunnelServer) isMediaRequest(requestData []byte) bool {
	if !s.streamConfig.EnableMediaOptimization {
		return false
	}

	requestStr := string(requestData)

	// Check for media file extensions from config
	for _, ext := range s.streamConfig.MediaExtensions {
		if strings.Contains(requestStr, ext) {
			return true
		}
	}

	// Check for Range requests (common for video streaming)
	if strings.Contains(requestStr, "Range:") {
		return true
	}

	// Check for media paths from config
	for _, path := range s.streamConfig.MediaPaths {
		if strings.Contains(requestStr, path) {
			return true
		}
	}

	return false
}



// proxyMediaRequest handles media requests with optimized streaming through the tunnel
func (s *TunnelServer) proxyMediaRequest(domain string, clientConn net.Conn, requestData []byte, requestBody io.Reader) {
	// Get the tunnel connection with validation
	tunnelConn := s.connections.GetHTTPConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No tunnel connection found for media request to domain: %s", domain)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Tunnel not connected")
		return
	}

	// Validate connection is still alive before using it
	if tunnelConn.GetConn() == nil {
		s.logger.Error("Tunnel connection is closed for domain: %s", domain)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Tunnel connection closed")
		return
	}

	// Quick connection validation for media requests
	tunnelConn.GetConn().SetDeadline(time.Now().Add(1 * time.Second))
	if _, err := tunnelConn.GetConn().Write([]byte{}); err != nil {
		s.logger.Error("Media tunnel connection failed validation for domain: %s, error: %v", domain, err)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		// Try to get another connection immediately
		if retryConn := s.connections.GetHTTPConnection(domain); retryConn != nil && retryConn != tunnelConn {
			s.logger.Info("Switching to backup connection for media request after validation failure")
			tunnelConn = retryConn
		} else {
			s.writeHTTPError(clientConn, 502, "Bad Gateway - No healthy media connections available")
			return
		}
	}
	tunnelConn.GetConn().SetDeadline(time.Time{})

	s.logger.Info("[MEDIA PROXY] Starting media request for domain: %s", domain)

	// Increment request count for this specific connection
	tunnelConn.IncrementRequestCount()

	// Lock the tunnel connection for proper HTTP/1.1 sequencing
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[MEDIA PROXY] Error writing request headers to tunnel: %v", err)
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Failed to write to media tunnel")
		return
	}

	// Copy request body if present
	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[MEDIA PROXY] Error writing request body to tunnel: %v", err)
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
			s.writeHTTPError(clientConn, 502, "Bad Gateway - Failed to write body to media tunnel")
			return
		}
	}

	s.logger.Info("[MEDIA PROXY] Reading response from tunnel...")

	// Set a read timeout to prevent hanging - use configurable media timeout
	mediaTimeout := s.streamConfig.MediaTimeout
	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(mediaTimeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{}) // Clear timeout

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())

			// Parse the response with error handling
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[MEDIA PROXY] Error reading response from tunnel: %v", err)
		// Connection is corrupted, remove it from pool
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)

		// Try to get a fresh connection for retry (prioritize retryable errors)
		if s.isRetryableConnectionError(err) && s.connections.GetHTTPPoolSize(domain) > 1 {
			s.logger.Info("[MEDIA PROXY] Retryable error detected, attempting retry with fresh connection")
			// Note: Let the defer handle the unlock - don't unlock manually here
			s.retryWithFreshConnection(domain, clientConn, requestData, requestBody)
			return
		}

		s.writeHTTPError(clientConn, 502, "Bad Gateway - Media tunnel response error")
		return
	}

	s.logger.Info("[MEDIA PROXY] Received response: %s", response.Status)

	// Write the response back to the client with optimized streaming and client disconnection detection
	clientWriter := bufio.NewWriter(clientConn)

	// Check if client is still connected before writing large response
	if s.isClientDisconnected(clientConn) {
		s.logger.Debug("[MEDIA PROXY] Client disconnected before response write, aborting media stream")
		return
	}

	if err := response.Write(clientWriter); err != nil {
		// This is often normal - client may close connection early during gallery navigation
		if s.isClientDisconnectionError(err) {
			s.logger.Debug("[MEDIA PROXY] Client closed connection during streaming (likely navigated away): %v", err)
		} else {
			s.logger.Debug("[MEDIA PROXY] Error writing response to client: %v", err)
		}
		return
	}

	if err := clientWriter.Flush(); err != nil {
		if s.isClientDisconnectionError(err) {
			s.logger.Debug("[MEDIA PROXY] Client disconnected during flush (likely navigated away): %v", err)
		} else {
			s.logger.Debug("[MEDIA PROXY] Error flushing response: %v", err)
		}
		return
	}

	s.logger.Info("[MEDIA PROXY] Media streaming completed successfully")

	// CRITICAL: Check if we should recycle this connection after media streaming
	if s.shouldRecycleConnection(tunnelConn, response) {
		s.logger.Debug("[MEDIA PROXY] Recycling tunnel connection after media request completion")
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		go tunnelConn.Close() // Close asynchronously to avoid blocking
	}
}

// writeHTTPError writes a proper HTTP error response
func (s *TunnelServer) writeHTTPError(conn net.Conn, code int, message string) {
	statusText := "Bad Gateway"
	if code == 504 {
		statusText = "Gateway Timeout"
	}

	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Type: text/plain\r\n"+
		"Connection: close\r\n"+
		"\r\n"+
		"%s", code, statusText, message)

	conn.Write([]byte(response))
}

// isRetryableConnectionError checks if an error indicates a connection issue that should trigger a retry
func (s *TunnelServer) isRetryableConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common connection errors that indicate the connection is dead/corrupted
	retryableErrors := []string{
		"use of closed network connection",
		"connection reset by peer",
		"broken pipe",
		"i/o timeout",
		"unexpected EOF",
		"EOF",
		"connection refused",
		"no route to host",
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// shouldRecycleConnection determines if a tunnel connection should be closed and recycled
func (s *TunnelServer) shouldRecycleConnection(tunnelConn *TunnelConnection, response *http.Response) bool {
	// Always recycle on connection close header
	if response != nil {
		if connHeader := response.Header.Get("Connection"); strings.ToLower(connHeader) == "close" {
			return true
		}
	}

	// Recycle connections that have handled many requests to prevent staleness
	// Be more aggressive for media requests to handle gallery navigation better
	requestCount := tunnelConn.GetRequestCount()
	if requestCount > 30 { // Recycle after 30 requests (more aggressive)
		return true
	}

	// Recycle connections that have been alive for too long
	// Be more aggressive for media connections
	if time.Since(tunnelConn.GetCreatedAt()) > 5*time.Minute {
		return true
	}

	// Recycle on certain HTTP status codes that might indicate connection issues
	if response != nil {
		switch response.StatusCode {
		case 502, 503, 504: // Bad Gateway, Service Unavailable, Gateway Timeout
			return true
		}
	}

	return false
}

	// recycleOldConnections proactively recycles connections that might be getting stuck
func (s *TunnelServer) recycleOldConnections(domain string) {
	connections := s.connections.GetAllHTTPConnections(domain)
	recycledCount := 0

	for _, conn := range connections {
		// Be more aggressive for potentially stuck connections
		// Especially important for image gallery navigation where connections can get stuck
		age := time.Since(conn.GetCreatedAt())
		requests := conn.GetRequestCount()

		shouldRecycle := false
		reason := ""

		if age > 3*time.Minute {
			shouldRecycle = true
			reason = "age"
		} else if requests > 20 {
			shouldRecycle = true
			reason = "request_count"
		}

		if shouldRecycle {
			s.logger.Info("[RECYCLE] Proactively recycling %s connection (age: %v, requests: %d)",
				reason, age, requests)
			s.connections.RemoveSpecificHTTPConnection(domain, conn)
			go conn.Close()
			recycledCount++
		}
	}

	if recycledCount > 0 {
		s.logger.Info("[RECYCLE] Proactively recycled %d connections for domain %s", recycledCount, domain)
	}
}

// isClientDisconnected checks if the client connection is still active (simplified for HTTP)
func (s *TunnelServer) isClientDisconnected(conn net.Conn) bool {
	// For HTTP requests, we can't reliably test without interfering with the stream
	// Just return false and rely on write error detection instead
	return false
}

// isClientDisconnectionError checks if an error indicates client disconnection
func (s *TunnelServer) isClientDisconnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common client disconnection error patterns
	disconnectionErrors := []string{
		"broken pipe",
		"connection reset by peer",
		"write: connection reset by peer",
		"use of closed network connection",
		"client disconnected",
		"connection closed",
		"EOF",
	}

	for _, disconnectionErr := range disconnectionErrors {
		if strings.Contains(errStr, disconnectionErr) {
			return true
		}
	}

	return false
}

// Connection error handling is now done through connection pool management

// These old retry methods have been replaced by retryWithFreshConnection

// ProxyWebSocketConnection handles WebSocket upgrade and bidirectional forwarding
func (s *TunnelServer) ProxyWebSocketConnection(domain string, clientConn net.Conn, r *http.Request) {
	defer clientConn.Close()

	tunnelConn := s.connections.GetWebSocketConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No WebSocket tunnel connection found for domain: %s", domain)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - WebSocket tunnel not connected")
		return
	}

	s.logger.Debug("[WEBSOCKET DEBUG] Starting WebSocket proxy for domain: %s", domain)

	// Build the WebSocket upgrade request
	var requestData strings.Builder

	// Add request line
	requestData.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, r.URL.RequestURI()))

	// Add Host header first
	requestData.WriteString(fmt.Sprintf("Host: %s\r\n", r.Host))

	// Add all headers (WebSocket upgrade headers are critical)
	for key, values := range r.Header {
		if key != "Host" { // Skip Host as we already added it
			for _, value := range values {
				requestData.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
			}
		}
	}

	// Add empty line to separate headers from body
	requestData.WriteString("\r\n")

	// Get the request as bytes
	requestBytes := []byte(requestData.String())
	// s.logger.Debug("[WEBSOCKET DEBUG] Forwarding WebSocket upgrade request:\n%s", requestData.String())

	// Lock the tunnel connection for the upgrade handshake
	tunnelConn.Lock()

	// Send the upgrade request to the tunnel
	if _, err := tunnelConn.GetConn().Write(requestBytes); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade request to tunnel: %v", err)
		tunnelConn.Unlock()
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Debug("[WEBSOCKET DEBUG] Sent WebSocket upgrade request to tunnel")

	// Read the upgrade response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error reading upgrade response from tunnel: %v", err)
		tunnelConn.Unlock()
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Debug("[WEBSOCKET DEBUG] Received upgrade response: %s", response.Status)

	// Write the upgrade response back to the client
	clientWriter := bufio.NewWriter(clientConn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade response to client: %v", err)
		tunnelConn.Unlock()
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error flushing upgrade response: %v", err)
		tunnelConn.Unlock()
		return
	}

	// Check if the upgrade was successful (101 Switching Protocols)
	if response.StatusCode != 101 {
		s.logger.Error("[WEBSOCKET DEBUG] WebSocket upgrade failed with status: %d", response.StatusCode)
		tunnelConn.Unlock()
		return
	}

	s.logger.Debug("[WEBSOCKET DEBUG] WebSocket upgrade successful, starting bidirectional forwarding")

	// Unlock the tunnel connection after successful upgrade
	// WebSocket data forwarding doesn't need the lock since it's bidirectional copying
	tunnelConn.Unlock()

	// Start bidirectional copying
	errChan := make(chan error, 2)

	// Copy from client to tunnel
	go func() {
		_, err := io.Copy(tunnelConn.GetConn(), clientConn)
		errChan <- err
	}()

	// Copy from tunnel to client
	go func() {
		_, err := io.Copy(clientConn, tunnelConn.GetConn())
		errChan <- err
	}()

	// Wait for either direction to close or error
	err = <-errChan
	if err != nil {
		s.logger.Debug("[WEBSOCKET DEBUG] WebSocket connection closed: %v", err)
	} else {
		s.logger.Debug("[WEBSOCKET DEBUG] WebSocket connection closed normally")
	}

	s.logger.Debug("[WEBSOCKET DEBUG] WebSocket proxy completed")
}