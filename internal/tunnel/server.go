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
	"runtime"
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

	// Circuit breaker for cascade failure prevention
	recentTimeouts  int64    // Recent timeout count
	lastTimeoutTime time.Time // Last timeout occurrence
}

// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository, tunnelService interfaces.TunnelService) *TunnelServer {
	// Create secure server TLS configuration
	serverTLSConfig, err := CreateSecureServerTLSConfig("/app/certs/tunnel.crt", "/app/certs/tunnel.key", "/app/certs/ca.crt")
	if err != nil {
		logging.GetGlobalLogger().Warn("Failed to create secure TLS config, using fallback: %v", err)
		// Fallback configuration for compatibility
		serverTLSConfig = &tls.Config{
			InsecureSkipVerify: true,
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
		}
	} else {
		logging.GetGlobalLogger().Info("🔐 TCP Server using PRODUCTION-GRADE TLS with mutual authentication")
	}

	return &TunnelServer{
		logger:       logging.GetGlobalLogger(),
		connections:  NewConnectionManager(),
		streamConfig: DefaultStreamingConfig(),
		tlsConfig:    serverTLSConfig,
		tokenRepo:    tokenRepo,
		tunnelRepo:   tunnelRepo,
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

// ProxyConnection handles proxying with hybrid approach: hot pool + on-demand creation
func (s *TunnelServer) ProxyConnection(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	defer conn.Close()

	// Performance monitoring
	atomic.AddInt64(&s.requestCount, 1)
	concurrent := atomic.AddInt64(&s.concurrentReqs, 1)
	defer atomic.AddInt64(&s.concurrentReqs, -1)

	// Log performance metrics every 10 requests and perform cleanup
	if atomic.LoadInt64(&s.requestCount)%10 == 0 {
		poolSize := s.connections.GetHTTPPoolSize(domain)
		hits := atomic.LoadInt64(&s.poolHits)
		misses := atomic.LoadInt64(&s.poolMisses)

		// Get memory statistics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Calculate actual connection memory overhead
		connOverheadMB := s.getConnectionMemoryOverhead()

		// Total system memory usage
		totalMemoryMB := float64(memStats.Alloc) / 1024.0 / 1024.0

		// Project costs for different pool sizes
		projected50MB := connOverheadMB * 50
		projected100MB := connOverheadMB * 100

		// Perform periodic cleanup every 5 minutes (less aggressive for stability)
		now := time.Now()
		if now.Sub(s.lastCleanup) > 5*time.Minute {
			cleanupStats := s.connections.CleanupDeadConnections()
			if len(cleanupStats) > 0 {
				s.logger.Info("[CLEANUP] Removed dead connections: %v", cleanupStats)
			}

			// Also proactively recycle old connections
			s.recycleOldConnections(domain)
			s.lastCleanup = now
		}

		recentTimeouts := atomic.LoadInt64(&s.recentTimeouts)
		s.logger.Info("[PERF] Requests: %d, Concurrent: %d, Hot Pool: %d, Hits: %d, Misses: %d, Timeouts: %d",
			atomic.LoadInt64(&s.requestCount), concurrent, poolSize, hits, misses, recentTimeouts)
		s.logger.Info("[MEMORY] Total: %.1fMB, Per-Conn: ~%.2fMB, Projected-50: %.1fMB, Projected-100: %.1fMB, GC: %d",
			totalMemoryMB, connOverheadMB, projected50MB, projected100MB, memStats.NumGC)
	}

	// HYBRID APPROACH: Try hot pool first, create on-demand if needed
	var tunnelConn *TunnelConnection
	var isOnDemand bool = false

	// Step 1: Try to get from hot pool (fast path)
	tunnelConn = s.connections.GetHTTPConnection(domain)
	if tunnelConn != nil {
		// Quick health check on hot pool connection
		if tunnelConn.GetConn() != nil {
			tunnelConn.GetConn().SetDeadline(time.Now().Add(100 * time.Millisecond))
			if _, err := tunnelConn.GetConn().Write([]byte{}); err == nil {
				atomic.AddInt64(&s.poolHits, 1)
				s.logger.Debug("[HYBRID] Using hot pool connection")
				tunnelConn.GetConn().SetDeadline(time.Time{})
			} else {
				// Hot pool connection is dead, remove and try on-demand
				s.logger.Debug("[HYBRID] Hot pool connection failed health check, removing")
				s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
				tunnelConn = nil
			}
		} else {
			tunnelConn = nil
		}
	}

	// Step 2: Create on-demand connection if hot pool failed
	if tunnelConn == nil {
		atomic.AddInt64(&s.poolMisses, 1)
		s.logger.Info("[HYBRID] Hot pool empty/unhealthy, creating on-demand connection (concurrent: %d)", concurrent)

		freshConn, err := s.createFreshTunnelConnection(domain)
		if err != nil {
			s.logger.Error("[HYBRID] Failed to create on-demand tunnel: %v", err)

			// Enhanced fallback: Try one more time after a brief delay
			s.logger.Info("[HYBRID] Attempting enhanced fallback after brief delay...")
			time.Sleep(50 * time.Millisecond)

			// Check if any new connections have appeared
			retryConn := s.connections.GetHTTPConnection(domain)
			if retryConn != nil {
				s.logger.Info("[HYBRID] Enhanced fallback successful - found new connection")
				tunnelConn = retryConn
				isOnDemand = false
			} else {
				s.logger.Error("[HYBRID] Enhanced fallback failed - no connections available")
				s.writeHTTPError(conn, 502, "Bad Gateway - No tunnel connections available")
				return
			}
		} else {
			tunnelConn = freshConn
			isOnDemand = true
			s.logger.Debug("[HYBRID] Created fresh on-demand connection")
		}
	}

	// Increment request count for this specific connection
	tunnelConn.IncrementRequestCount()

	// Check if this is a media/video request that should use optimized handling
	isMediaRequest := s.isMediaRequest(requestData)

	if isMediaRequest {
		// For media requests, use optimized handling
		s.proxyMediaRequestHybrid(domain, conn, requestData, requestBody, tunnelConn, isOnDemand)
		return
	}

	// CRITICAL: Lock this specific connection for HTTP/1.1 request-response cycle
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	// Double-check connection after acquiring lock
	if tunnelConn.GetConn() == nil {
		s.logger.Error("[HYBRID] Tunnel connection closed after lock acquisition for domain: %s", domain)
		s.writeHTTPError(conn, 502, "Bad Gateway - Tunnel connection closed")
		return
	}

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[HYBRID] Error writing request headers to tunnel: %v", err)

		// For on-demand connections, don't retry - just fail
		if isOnDemand {
			s.writeHTTPError(conn, 502, "Bad Gateway - On-demand tunnel write failed")
			return
		}

		// For hot pool connections, remove and try creating on-demand
		s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		s.logger.Info("[HYBRID] Hot pool connection failed, trying on-demand fallback")

		// Quick on-demand fallback
		s.proxyWithOnDemandFallback(domain, conn, requestData, requestBody)
		return
	}

	// Copy request body if present
	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[HYBRID] Error writing request body to tunnel: %v", err)
			if !isOnDemand {
				s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
			}
			s.writeHTTPError(conn, 502, "Bad Gateway - Failed to write body to tunnel")
			return
		}
	}

	// Set a read timeout - use adaptive timeout based on circuit breaker
	regularTimeout := s.getAdaptiveTimeout(false) // false = not media request
	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(regularTimeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{}) // Clear timeout

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())

	// Parse the response with better error handling
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[HYBRID] Error reading response from tunnel: %v", err)

		// Record timeout for circuit breaker if it's a timeout error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			s.recordTimeout()
		}

		// Remove from hot pool if it was a hot pool connection
		if !isOnDemand {
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		}

		s.writeHTTPError(conn, 502, "Bad Gateway - Tunnel response error")
		return
	}

	// Write the response back to the client
	clientWriter := bufio.NewWriter(conn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Debug("[HYBRID] Error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Debug("[HYBRID] Error flushing response: %v", err)
		return
	}

	// HYBRID CLEANUP: Aggressive connection management like frp
	if isOnDemand {
		// On-demand connections: ALWAYS close (like frp)
		s.logger.Debug("[HYBRID] Closing on-demand connection")
		go tunnelConn.Close()
	} else {
		// Hot pool connections: Close if getting old/heavily used
		if !s.shouldKeepInHotPool(tunnelConn, response) {
			s.logger.Debug("[HYBRID] Hot pool connection getting stale, removing")
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
			go tunnelConn.Close()
		} else {
			s.logger.Debug("[HYBRID] Keeping fresh connection in hot pool")
		}
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

	// Set a read timeout for retry - use adaptive timeout
	retryTimeout := s.getAdaptiveTimeout(false) // false = not media request
	retryTunnelConn.GetConn().SetReadDeadline(time.Now().Add(retryTimeout))
	defer retryTunnelConn.GetConn().SetReadDeadline(time.Time{})

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(retryTunnelConn.GetConn())

	// Parse the response
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Retry failed - error reading response: %v", err)

		// Record timeout for circuit breaker if retry also timed out
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			s.recordTimeout()
		}

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

	// Set a read timeout to prevent hanging - use adaptive timeout for media
	// Under high load (stress), use much shorter timeouts to clear queue faster
	currentConcurrent := atomic.LoadInt64(&s.concurrentReqs)
	poolSize := int64(s.connections.GetHTTPPoolSize(domain)) // Use actual pool size

	var mediaTimeout time.Duration
	if currentConcurrent > poolSize*2 { // If concurrent > 2x pool size, we're in overload
		mediaTimeout = 3 * time.Second // Very aggressive timeout under stress
		s.logger.Debug("[MEDIA PROXY] System under stress (%d concurrent vs %d pool), using aggressive 3s timeout", currentConcurrent, poolSize)
	} else if currentConcurrent > poolSize { // If concurrent > pool size, we're stressed
		mediaTimeout = 8 * time.Second // Moderate timeout under stress
		s.logger.Debug("[MEDIA PROXY] System stressed (%d concurrent vs %d pool), using moderate 8s timeout", currentConcurrent, poolSize)
	} else {
		mediaTimeout = s.getAdaptiveTimeout(true) // Normal timeout
	}

	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(mediaTimeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{}) // Clear timeout

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())

			// Parse the response with error handling
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[MEDIA PROXY] Error reading response from tunnel: %v", err)

		// Record timeout for circuit breaker if it's a timeout error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			s.recordTimeout()

			// Under stress, don't retry timeouts - just fail fast to clear queue
			currentConcurrent := atomic.LoadInt64(&s.concurrentReqs)
			poolSize := int64(s.connections.GetHTTPPoolSize(domain))
			if currentConcurrent > poolSize { // If more concurrent than pool size
				s.logger.Debug("[MEDIA PROXY] Under stress (%d concurrent vs %d pool), failing fast on timeout to clear queue", currentConcurrent, poolSize)
				s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
				s.writeHTTPError(clientConn, 504, "Gateway Timeout - System overloaded")
				return
			}
		}

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

	// CRITICAL: Check if client disconnected during media processing (fast clicking scenario)
	clientConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	testBuf := make([]byte, 1)
	_, clientErr := clientConn.Read(testBuf)
	clientConn.SetReadDeadline(time.Time{}) // Clear deadline

	// If client disconnected, abort immediately to free up the connection
	if clientErr != nil && !strings.Contains(clientErr.Error(), "timeout") && !strings.Contains(clientErr.Error(), "deadline") {
		s.logger.Debug("[MEDIA PROXY] Client disconnected during processing, aborting to free connection")
		return
	}

	// Write the response back to the client with optimized streaming
	clientWriter := bufio.NewWriter(clientConn)

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

	// CRITICAL: Always validate connection state after media use
	if !s.isConnectionCleanForReuse(tunnelConn, response) {
		s.logger.Debug("[MEDIA PROXY] Connection not clean after media request, removing from pool")
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

// isConnectionCleanForReuse determines if a tunnel connection is safe to reuse
func (s *TunnelServer) isConnectionCleanForReuse(tunnelConn *TunnelConnection, response *http.Response) bool {
	// NEVER reuse connections with "Connection: close" header
	if response != nil {
		if connHeader := response.Header.Get("Connection"); strings.ToLower(connHeader) == "close" {
			s.logger.Debug("[CONNECTION] Connection marked for close by server")
			return false
		}
	}

	// NEVER reuse connections that have handled too many requests
	requestCount := tunnelConn.GetRequestCount()
	if requestCount > 50 { // Allow more requests per connection to prevent cascade failures
		s.logger.Debug("[CONNECTION] Connection has handled %d requests, retiring", requestCount)
		return false
	}

	// NEVER reuse connections older than 10 minutes (less aggressive for fast clicking)
	if time.Since(tunnelConn.GetCreatedAt()) > 10*time.Minute {
		s.logger.Debug("[CONNECTION] Connection is %v old, retiring", time.Since(tunnelConn.GetCreatedAt()))
		return false
	}

	// NEVER reuse connections after error status codes
	if response != nil {
		switch response.StatusCode {
		case 502, 503, 504: // Bad Gateway, Service Unavailable, Gateway Timeout
			s.logger.Debug("[CONNECTION] Error status code %d, retiring connection", response.StatusCode)
			return false
		}
	}

	// NEVER reuse if connection appears unhealthy
	if tunnelConn.GetConn() == nil {
		s.logger.Debug("[CONNECTION] Connection is nil, cannot reuse")
		return false
	}

	// Test connection health with a minimal write
	tunnelConn.GetConn().SetDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := tunnelConn.GetConn().Write([]byte{}); err != nil {
		s.logger.Debug("[CONNECTION] Connection failed health check: %v", err)
		tunnelConn.GetConn().SetDeadline(time.Time{})
		return false
	}
	tunnelConn.GetConn().SetDeadline(time.Time{})

	s.logger.Debug("[CONNECTION] Connection passed all health checks, safe for reuse")
	return true
}

// getAdaptiveTimeout returns timeout based on recent failures (circuit breaker logic)
func (s *TunnelServer) getAdaptiveTimeout(isMedia bool) time.Duration {
	// Check if we're in a failure cascade (many recent timeouts)
	recentTimeouts := atomic.LoadInt64(&s.recentTimeouts)
	timeSinceLastTimeout := time.Since(s.lastTimeoutTime)

	// Reset timeout counter if it's been quiet for a while
	if timeSinceLastTimeout > 30*time.Second {
		atomic.StoreInt64(&s.recentTimeouts, 0)
		recentTimeouts = 0
	}

	var baseTimeout time.Duration
	if isMedia {
		baseTimeout = s.streamConfig.MediaTimeout
	} else {
		baseTimeout = s.streamConfig.RegularTimeout
	}

	// If we're seeing many timeouts, be more aggressive with shorter timeouts
	if recentTimeouts > 3 {
		aggressiveTimeout := baseTimeout / 3 // Much shorter timeout during failures
		s.logger.Debug("[CIRCUIT BREAKER] Using aggressive timeout %v due to %d recent timeouts", aggressiveTimeout, recentTimeouts)
		return aggressiveTimeout
	}

	return baseTimeout
}

// recordTimeout records a timeout for circuit breaker logic
func (s *TunnelServer) recordTimeout() {
	atomic.AddInt64(&s.recentTimeouts, 1)
	s.lastTimeoutTime = time.Now()
	timeouts := atomic.LoadInt64(&s.recentTimeouts)
	s.logger.Debug("[CIRCUIT BREAKER] Recorded timeout #%d", timeouts)
}

	// recycleOldConnections proactively recycles connections that might be getting stuck
func (s *TunnelServer) recycleOldConnections(domain string) {
	connections := s.connections.GetAllHTTPConnections(domain)
	recycledCount := 0

	for _, conn := range connections {
		// Be VERY aggressive for potentially stuck connections
		// Especially important for image gallery navigation where connections can get stuck
		age := time.Since(conn.GetCreatedAt())
		requests := conn.GetRequestCount()

		shouldRecycle := false
		reason := ""

		if age > 15*time.Minute {
			shouldRecycle = true
			reason = "age"
		} else if requests > 100 {
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

// getConnectionMemoryOverhead estimates memory overhead per connection
func (s *TunnelServer) getConnectionMemoryOverhead() float64 {
	// Estimate memory per connection:
	// - TCP connection buffers (kernel): ~16KB each (read + write)
	// - TLS buffers: ~32KB each
	// - Application buffers: MediaBufferSize + RegularBufferSize
	// - Connection struct + metadata: ~1KB

	tcpBuffers := 16.0 * 1024  // 16KB kernel buffers
	tlsBuffers := 32.0 * 1024  // 32KB TLS buffers
	appBuffers := float64(s.streamConfig.MediaBufferSize + s.streamConfig.RegularBufferSize)
	metadata := 1.0 * 1024     // 1KB struct overhead

	totalBytesPerConn := tcpBuffers + tlsBuffers + appBuffers + metadata
	return totalBytesPerConn / 1024.0 / 1024.0 // Convert to MB
}

// ProxyConnectionOnTheFly handles proxying with fresh tunnel per request
func (s *TunnelServer) ProxyConnectionOnTheFly(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	defer conn.Close()

	// Performance monitoring
	atomic.AddInt64(&s.requestCount, 1)
	concurrent := atomic.AddInt64(&s.concurrentReqs, 1)
	defer atomic.AddInt64(&s.concurrentReqs, -1)

	s.logger.Info("[ON-THE-FLY] Creating fresh tunnel for request to domain: %s (concurrent: %d)", domain, concurrent)

	// Create a fresh tunnel connection for this request only
	tunnelConn, err := s.createFreshTunnelConnection(domain)
	if err != nil {
		s.logger.Error("[ON-THE-FLY] Failed to create fresh tunnel: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Failed to create tunnel")
		return
	}

	// Ensure we always close this connection
	defer func() {
		s.logger.Debug("[ON-THE-FLY] Closing fresh tunnel connection")
		tunnelConn.Close()
	}()

	s.logger.Debug("[ON-THE-FLY] Fresh tunnel established, processing request")

	// Check if this is a media/video request
	isMediaRequest := s.isMediaRequest(requestData)
	timeout := s.streamConfig.RegularTimeout
	if isMediaRequest {
		timeout = s.streamConfig.MediaTimeout
		s.logger.Info("[ON-THE-FLY] Using media timeout: %v", timeout)
	}

	// Set timeout for the entire request-response cycle
	tunnelConn.GetConn().SetDeadline(time.Now().Add(timeout))
	defer tunnelConn.GetConn().SetDeadline(time.Time{})

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[ON-THE-FLY] Error writing request headers: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Failed to write request")
		return
	}

	// Copy request body if present
	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[ON-THE-FLY] Error writing request body: %v", err)
			s.writeHTTPError(conn, 502, "Bad Gateway - Failed to write body")
			return
		}
	}

	s.logger.Debug("[ON-THE-FLY] Request sent, reading response...")

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[ON-THE-FLY] Error reading response: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Failed to read response")
		return
	}

	s.logger.Debug("[ON-THE-FLY] Response received: %s, writing to client", response.Status)

	// Write the response back to the client
	clientWriter := bufio.NewWriter(conn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Debug("[ON-THE-FLY] Error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Debug("[ON-THE-FLY] Error flushing response: %v", err)
		return
	}

	s.logger.Debug("[ON-THE-FLY] Request completed successfully, tunnel will be closed")
}

// createFreshTunnelConnection creates a new tunnel connection on-demand
func (s *TunnelServer) createFreshTunnelConnection(domain string) (*TunnelConnection, error) {
	// CURRENT LIMITATION: This is still a fallback to existing pool
	// TODO: Implement true on-demand connection creation by connecting back to client

	// Try to get from pool first
	tunnelConn := s.connections.GetHTTPConnection(domain)
	if tunnelConn != nil {
		s.logger.Debug("[HYBRID] Got existing connection from pool for on-demand request")
		return tunnelConn, nil
	}

	// Pool is empty - wait a brief moment for new connections to be established
	s.logger.Info("[HYBRID] Pool empty, waiting briefly for new connections...")
	time.Sleep(100 * time.Millisecond)

	// Try again after brief wait
	tunnelConn = s.connections.GetHTTPConnection(domain)
	if tunnelConn != nil {
		s.logger.Debug("[HYBRID] Got connection after brief wait")
		return tunnelConn, nil
	}

	// Still no connections available
	// In a real on-the-fly implementation, you would:
	// 1. Look up the client IP/port for this domain from tunnel repository
	// 2. Establish a new TCP connection to that client
	// 3. Perform handshake with client to create fresh tunnel
	// 4. Return the fresh connection (not added to pool)

	s.logger.Error("[HYBRID] No connections available and true on-demand not yet implemented")
	return nil, fmt.Errorf("no tunnel available for domain: %s (pool empty, true on-demand not implemented)", domain)
}

// shouldKeepInHotPool determines if a connection should stay in the hot pool (less aggressive to maintain stability)
func (s *TunnelServer) shouldKeepInHotPool(tunnelConn *TunnelConnection, response *http.Response) bool {
	// Be LESS aggressive to maintain hot pool stability until true on-demand is implemented

	// NEVER keep connections with "Connection: close" header
	if response != nil {
		if connHeader := response.Header.Get("Connection"); strings.ToLower(connHeader) == "close" {
			s.logger.Debug("[HYBRID] Connection marked for close by server")
			return false
		}
	}

	// NEVER keep connections that have handled too many requests (increased from 3 to 15)
	requestCount := tunnelConn.GetRequestCount()
	if requestCount > 15 {
		s.logger.Debug("[HYBRID] Connection handled %d requests, too many for hot pool", requestCount)
		return false
	}

	// NEVER keep connections older than 5 minutes (increased from 30 seconds)
	if time.Since(tunnelConn.GetCreatedAt()) > 5*time.Minute {
		s.logger.Debug("[HYBRID] Connection is %v old, too old for hot pool", time.Since(tunnelConn.GetCreatedAt()))
		return false
	}

	// NEVER keep connections after server error status codes (4xx errors are OK)
	if response != nil && response.StatusCode >= 500 {
		s.logger.Debug("[HYBRID] Server error status code %d, removing from hot pool", response.StatusCode)
		return false
	}

	// NEVER keep if connection appears unhealthy
	if tunnelConn.GetConn() == nil {
		s.logger.Debug("[HYBRID] Connection is nil, cannot keep in hot pool")
		return false
	}

	s.logger.Debug("[HYBRID] Connection is suitable for hot pool (age: %v, requests: %d)",
		time.Since(tunnelConn.GetCreatedAt()), requestCount)
	return true
}

// proxyWithOnDemandFallback handles fallback when hot pool connection fails
func (s *TunnelServer) proxyWithOnDemandFallback(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	s.logger.Info("[HYBRID] Executing on-demand fallback")

	// Create fresh connection
	tunnelConn, err := s.createFreshTunnelConnection(domain)
	if err != nil {
		s.logger.Error("[HYBRID] Fallback failed to create tunnel: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Fallback failed")
		return
	}

	// Always close on-demand fallback connections
	defer func() {
		s.logger.Debug("[HYBRID] Closing fallback connection")
		tunnelConn.Close()
	}()

	// Lock and process the request
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	// Write request
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[HYBRID] Fallback failed to write headers: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Fallback write failed")
		return
	}

	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[HYBRID] Fallback failed to write body: %v", err)
			s.writeHTTPError(conn, 502, "Bad Gateway - Fallback body failed")
			return
		}
	}

	// Set timeout and read response
	timeout := s.getAdaptiveTimeout(false)
	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(timeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{})

	tunnelReader := bufio.NewReader(tunnelConn.GetConn())
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[HYBRID] Fallback failed to read response: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Fallback response failed")
		return
	}

	// Write response to client
	clientWriter := bufio.NewWriter(conn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Debug("[HYBRID] Fallback error writing to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Debug("[HYBRID] Fallback error flushing: %v", err)
		return
	}

	s.logger.Debug("[HYBRID] Fallback completed successfully")
}

// proxyMediaRequestHybrid handles media requests with hybrid approach
func (s *TunnelServer) proxyMediaRequestHybrid(domain string, clientConn net.Conn, requestData []byte, requestBody io.Reader, tunnelConn *TunnelConnection, isOnDemand bool) {
	s.logger.Info("[HYBRID MEDIA] Starting media request for domain: %s (on-demand: %t)", domain, isOnDemand)

	// Increment request count for this specific connection
	tunnelConn.IncrementRequestCount()

	// Lock the tunnel connection for proper HTTP/1.1 sequencing
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.GetConn().Write(requestData); err != nil {
		s.logger.Error("[HYBRID MEDIA] Error writing request headers: %v", err)
		if !isOnDemand {
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		}
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Failed to write to media tunnel")
		return
	}

	// Copy request body if present
	if requestBody != nil {
		if _, err := io.Copy(tunnelConn.GetConn(), requestBody); err != nil {
			s.logger.Error("[HYBRID MEDIA] Error writing request body: %v", err)
			if !isOnDemand {
				s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
			}
			s.writeHTTPError(clientConn, 502, "Bad Gateway - Failed to write body to media tunnel")
			return
		}
	}

	s.logger.Info("[HYBRID MEDIA] Reading response from tunnel...")

	// Set timeout based on current load
	currentConcurrent := atomic.LoadInt64(&s.concurrentReqs)
	poolSize := int64(s.connections.GetHTTPPoolSize(domain))

	var mediaTimeout time.Duration
	if currentConcurrent > poolSize*2 {
		mediaTimeout = 3 * time.Second
		s.logger.Debug("[HYBRID MEDIA] System under stress (%d concurrent vs %d pool), using aggressive 3s timeout", currentConcurrent, poolSize)
	} else if currentConcurrent > poolSize {
		mediaTimeout = 8 * time.Second
		s.logger.Debug("[HYBRID MEDIA] System stressed (%d concurrent vs %d pool), using moderate 8s timeout", currentConcurrent, poolSize)
	} else {
		mediaTimeout = s.getAdaptiveTimeout(true)
	}

	tunnelConn.GetConn().SetReadDeadline(time.Now().Add(mediaTimeout))
	defer tunnelConn.GetConn().SetReadDeadline(time.Time{})

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.GetConn())
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[HYBRID MEDIA] Error reading response: %v", err)

		// Record timeout for circuit breaker if it's a timeout error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			s.recordTimeout()

			// Under stress, just fail fast
			if currentConcurrent > poolSize*4 { // Proportional to hot pool size (5*4=20, but scales)
				s.logger.Debug("[HYBRID MEDIA] Under stress, failing fast on timeout")
				if !isOnDemand {
					s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
				}
				s.writeHTTPError(clientConn, 504, "Gateway Timeout - System overloaded")
				return
			}
		}

		// Remove from hot pool if it was a hot pool connection
		if !isOnDemand {
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
		}

		s.writeHTTPError(clientConn, 502, "Bad Gateway - Media tunnel response error")
		return
	}

	s.logger.Info("[HYBRID MEDIA] Received response: %s", response.Status)

	// Quick client disconnection check
	clientConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	testBuf := make([]byte, 1)
	_, clientErr := clientConn.Read(testBuf)
	clientConn.SetReadDeadline(time.Time{})

	if clientErr != nil && !strings.Contains(clientErr.Error(), "timeout") && !strings.Contains(clientErr.Error(), "deadline") {
		s.logger.Debug("[HYBRID MEDIA] Client disconnected during processing, aborting")
		return
	}

	// Write the response back to the client
	clientWriter := bufio.NewWriter(clientConn)
	if err := response.Write(clientWriter); err != nil {
		if s.isClientDisconnectionError(err) {
			s.logger.Debug("[HYBRID MEDIA] Client closed connection during streaming: %v", err)
		} else {
			s.logger.Debug("[HYBRID MEDIA] Error writing response to client: %v", err)
		}
		return
	}

	if err := clientWriter.Flush(); err != nil {
		if s.isClientDisconnectionError(err) {
			s.logger.Debug("[HYBRID MEDIA] Client disconnected during flush: %v", err)
		} else {
			s.logger.Debug("[HYBRID MEDIA] Error flushing response: %v", err)
		}
		return
	}

	s.logger.Info("[HYBRID MEDIA] Media streaming completed successfully")

	// HYBRID CLEANUP for media requests
	if isOnDemand {
		// On-demand connections: ALWAYS close
		s.logger.Debug("[HYBRID MEDIA] Closing on-demand media connection")
		go tunnelConn.Close()
	} else {
		// Hot pool connections: Be very aggressive about media connections
		if !s.shouldKeepInHotPool(tunnelConn, response) {
			s.logger.Debug("[HYBRID MEDIA] Media connection not suitable for hot pool, removing")
			s.connections.RemoveSpecificHTTPConnection(domain, tunnelConn)
			go tunnelConn.Close()
		} else {
			s.logger.Debug("[HYBRID MEDIA] Keeping fresh media connection in hot pool")
		}
	}
}