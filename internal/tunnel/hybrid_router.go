package tunnel

import (
	"bufio"
	"fmt"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HybridTunnelRouter provides intelligent routing between gRPC and TCP tunnels
// This is the main entry point for all tunnel traffic - competing with Cloudflare!
type HybridTunnelRouter struct {
	// Core services
	grpcTunnel *GRPCTunnelServer // Handles HTTP traffic with unlimited concurrency
	tcpTunnel  *TunnelServer     // Handles WebSocket traffic (legacy)
	logger     *logging.Logger

	// Performance metrics
	totalRequests     int64
	grpcRequests      int64
	tcpRequests       int64
	websocketUpgrades int64
	routingErrors     int64
	timeoutErrors     int64

	// Configuration
	config *HybridRouterConfig

	// Usage aggregation
	usage UsageRecorder
	// Quotas
	quota QuotaChecker

	// Demand-based tunnel establishment
	pendingConnections     map[string][]*PendingWebSocketConnection
	pendingConnectionsMu   sync.RWMutex
	tunnelEstablishTimeout time.Duration
}

// PendingWebSocketConnection represents a WebSocket connection waiting for TCP tunnel establishment
type PendingWebSocketConnection struct {
	Domain      string
	Conn        net.Conn
	RequestData []byte
	RequestBody io.Reader
	ClientIP    string
	HTTPReq     *http.Request
	RequestID   string
	StartTime   time.Time
	DoneChan    chan bool
}

// HybridRouterConfig holds configuration for the hybrid router
type HybridRouterConfig struct {
	// Server addresses
	GRPCAddress string
	TCPAddress  string

	// Request classification
	ForceGRPCPaths []string // Paths that must use gRPC
	ForceTCPPaths  []string // Paths that must use TCP

	// Large file handling
	LargeFileExtensions []string // File extensions for large files (videos, etc.)
	MaxGRPCFileSize     int64    // Max file size for gRPC (bytes), larger files use TCP streaming
	LargeFilePaths      []string // URL patterns that likely contain large files

	// Performance settings
	EnableMetrics   bool
	MetricsInterval time.Duration

	// Security settings
	EnableRateLimit   bool
	MaxRequestsPerMin int
}

// DefaultHybridRouterConfig returns production-ready configuration
func DefaultHybridRouterConfig() *HybridRouterConfig {
	return &HybridRouterConfig{
		GRPCAddress:    ":4444",                                                                            // Different port for gRPC
		TCPAddress:     ":4443",                                                                            // Original port for TCP/WebSocket
		ForceGRPCPaths: []string{"/assets/", "/media/", "/static/"},                                        // Removed /api/ to allow WebSocket routing
		ForceTCPPaths:  []string{"/ws/", "/websocket/", "/socket.io/", "socket.io", "transport=websocket"}, // Enhanced WebSocket patterns

		// Large file handling - route big files to TCP streaming
		LargeFileExtensions: []string{".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4v", ".flv", ".wmv", // Videos
			".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", // Archives
			".iso", ".img", ".dmg", ".exe", ".msi", ".deb", ".rpm"}, // Large binaries
		MaxGRPCFileSize: 50 * 1024 * 1024,                                                   // 50MB - files larger than this use TCP streaming
		LargeFilePaths:  []string{"/video/", "/download/", "/file/", "/original/", "/raw/"}, // Paths likely to contain large files

		EnableMetrics:     true,
		MetricsInterval:   1 * time.Minute,
		EnableRateLimit:   true,
		MaxRequestsPerMin: 10000,
	}
}

// NewHybridTunnelRouter creates a new hybrid tunnel router
func NewHybridTunnelRouter(
	tokenRepo repository.TokenRepository,
	tunnelRepo repository.TunnelRepository,
	tunnelService interfaces.TunnelService,
	config *HybridRouterConfig,
) *HybridTunnelRouter {
	if config == nil {
		config = DefaultHybridRouterConfig()
	}

	router := &HybridTunnelRouter{
		logger:                 logging.GetGlobalLogger(),
		config:                 config,
		pendingConnections:     make(map[string][]*PendingWebSocketConnection),
		tunnelEstablishTimeout: 30 * time.Second, // 30 second timeout for tunnel establishment
	}

	// Create gRPC tunnel server (for HTTP traffic)
	grpcConfig := DefaultGRPCTunnelConfig()
	router.grpcTunnel = NewGRPCTunnelServer(tokenRepo, tunnelRepo, tunnelService, grpcConfig)

	// Create TCP tunnel server (for WebSocket traffic)
	router.tcpTunnel = NewServer(tokenRepo, tunnelRepo, tunnelService)

	// Set up TCP tunnel establishment callback
	router.tcpTunnel.SetTCPTunnelEstablishedCallback(router.OnTCPTunnelEstablished)

	return router
}

// SetUsageRecorder sets a usage recorder for both gRPC and TCP tunnel servers
func (r *HybridTunnelRouter) SetUsageRecorder(rec UsageRecorder) {
	r.usage = rec
	if r.grpcTunnel != nil {
		r.grpcTunnel.SetUsageRecorder(rec)
	}
	if r.tcpTunnel != nil {
		r.tcpTunnel.SetUsageRecorder(rec)
	}
}

// Grpc exposes the gRPC tunnel server (if needed by callers)
func (r *HybridTunnelRouter) Grpc() *GRPCTunnelServer { return r.grpcTunnel }

// Tcp exposes the TCP tunnel server (if needed by callers)
func (r *HybridTunnelRouter) Tcp() *TunnelServer { return r.tcpTunnel }

// QuotaChecker minimal interface to avoid tight coupling
// QuotaChecker is defined in quota.go and implemented by services

// SetQuotaChecker sets quota checker service into underlying servers
func (r *HybridTunnelRouter) SetQuotaChecker(q QuotaChecker) {
	r.quota = q
	if r.grpcTunnel != nil {
		r.grpcTunnel.SetQuotaChecker(q)
	}
	if r.tcpTunnel != nil {
		r.tcpTunnel.SetQuotaChecker(q)
	}
}

// Start starts both tunnel servers
func (r *HybridTunnelRouter) Start() error {
	r.logger.Info("Starting Hybrid Tunnel Router (Production-Grade)")

	// Start gRPC tunnel server
	if err := r.grpcTunnel.Start(r.config.GRPCAddress); err != nil {
		return fmt.Errorf("failed to start gRPC tunnel server: %w", err)
	}
	r.logger.Info("✓ gRPC Tunnel Server started on %s", r.config.GRPCAddress)

	// Start TCP tunnel server
	if err := r.tcpTunnel.Start(r.config.TCPAddress); err != nil {
		return fmt.Errorf("failed to start TCP tunnel server: %w", err)
	}
	r.logger.Info("✓ TCP Tunnel Server started on %s", r.config.TCPAddress)

	// Start metrics reporting
	if r.config.EnableMetrics {
		go r.reportMetrics()
	}

	r.logger.Info("🚀 Hybrid Tunnel Router started successfully - Ready to compete with Cloudflare!")
	return nil
}

// Stop gracefully stops both tunnel servers
func (r *HybridTunnelRouter) Stop() error {
	r.logger.Info("Stopping Hybrid Tunnel Router...")

	// Stop gRPC tunnel server
	if err := r.grpcTunnel.Stop(); err != nil {
		r.logger.Error("Error stopping gRPC tunnel server: %v", err)
	}

	// Stop TCP tunnel server
	if err := r.tcpTunnel.Stop(); err != nil {
		r.logger.Error("Error stopping TCP tunnel server: %v", err)
	}

	r.logger.Info("Hybrid Tunnel Router stopped")
	return nil
}

// ProxyConnection intelligently routes connections to the appropriate tunnel
func (r *HybridTunnelRouter) ProxyConnection(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	defer conn.Close()

	atomic.AddInt64(&r.totalRequests, 1)

	// Extract client IP for logging and security
	clientIP := r.extractClientIP(conn)

	// Parse the request to determine routing
	shouldUseTCP, httpMethod, requestPath := r.analyzeRequest(requestData)

	// Determine the actual request type
	isActualWebSocket := r.isWebSocketUpgrade(requestData)
	isLargeFile := r.isLargeFile(requestPath)

	r.logger.Debug("[HYBRID] Request from %s: %s %s (TCP: %t, WebSocket: %t, LargeFile: %t)",
		clientIP, httpMethod, requestPath, shouldUseTCP, isActualWebSocket, isLargeFile)

	// Route based on request type
	if shouldUseTCP {
		if isActualWebSocket {
			r.routeToTCPTunnel(domain, conn, requestData, requestBody, clientIP)
		} else {
			// Large file - route to gRPC chunked streaming for unlimited concurrency
			r.routeToGRPCChunkedStreaming(domain, conn, requestData, requestBody, clientIP, httpMethod, requestPath)
		}
	} else {
		r.routeToGRPCTunnel(domain, conn, requestData, requestBody, clientIP, httpMethod, requestPath)
	}
}

// routeToGRPCTunnel routes HTTP traffic to the gRPC tunnel
func (r *HybridTunnelRouter) routeToGRPCTunnel(domain string, conn net.Conn, requestData []byte, requestBody io.Reader, clientIP, method, path string) {
	atomic.AddInt64(&r.grpcRequests, 1)

	r.logger.Debug("[HYBRID→gRPC] Routing HTTP request: %s %s", method, path)

	// Check if gRPC tunnel is available
	if !r.grpcTunnel.IsTunnelActive(domain) {
		r.logger.Error("[HYBRID→gRPC] No active gRPC tunnel for domain: %s", domain)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 502, "Bad Gateway - gRPC tunnel not available")
		return
	}

	// Parse HTTP request from raw data
	httpReq, err := r.parseHTTPRequest(requestData, requestBody)
	if err != nil {
		r.logger.Error("[HYBRID→gRPC] Failed to parse HTTP request: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 400, "Bad Request - Invalid HTTP request")
		return
	}

	// Proxy through gRPC tunnel
	var response *http.Response
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		// Stream uploads to avoid 16MB gRPC limits
		response, err = r.grpcTunnel.ProxyHTTPRequestWithChunking(domain, httpReq, clientIP)
	default:
		// Fast path for GET/HEAD and small requests
		response, err = r.grpcTunnel.ProxyHTTPRequest(domain, httpReq, clientIP)
	}
	if err != nil {
		r.logger.Error("[HYBRID→gRPC] gRPC proxy error: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		if isTimeoutError(err) {
			atomic.AddInt64(&r.timeoutErrors, 1)
		}
		r.writeHTTPError(conn, 502, fmt.Sprintf("Bad Gateway - %v", err))
		return
	}

	// Write response back to client
	writer := bufio.NewWriter(conn)
	if err := response.Write(writer); err != nil {
		r.logger.Error("[HYBRID→gRPC] Error writing response: %v", err)
		return
	}

	if err := writer.Flush(); err != nil {
		r.logger.Error("[HYBRID→gRPC] Error flushing response: %v", err)
		return
	}

	r.logger.Debug("[HYBRID→gRPC] Request completed successfully")
}

// routeToTCPTunnel routes WebSocket traffic to the TCP tunnel
func (r *HybridTunnelRouter) routeToTCPTunnel(domain string, conn net.Conn, requestData []byte, requestBody io.Reader, clientIP string) {
	atomic.AddInt64(&r.tcpRequests, 1)
	atomic.AddInt64(&r.websocketUpgrades, 1)

	r.logger.Debug("[HYBRID→TCP] Routing WebSocket upgrade")

	// Parse HTTP request for WebSocket upgrade first (to avoid parsing twice)
	httpReq, err := r.parseHTTPRequest(requestData, requestBody)
	if err != nil {
		r.logger.Error("[HYBRID→TCP] Failed to parse WebSocket request: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 400, "Bad Request - Invalid WebSocket request")
		return
	}

	// Check if TCP tunnel is available - with health validation
	if !r.tcpTunnel.IsTunnelDomain(domain) {
		r.logger.Info("[HYBRID→TCP] No active TCP tunnel for domain: %s, requesting establishment...", domain)

		// Instead of returning 502, wait for tunnel establishment
		if r.waitForTCPTunnelEstablishment(domain, conn, requestData, requestBody, clientIP, httpReq) {
			// Tunnel was established, continue with WebSocket upgrade
			r.logger.Info("[HYBRID→TCP] TCP tunnel established successfully for domain: %s", domain)
		} else {
			// Timeout or error establishing tunnel
			r.logger.Error("[HYBRID→TCP] Failed to establish TCP tunnel for domain: %s", domain)
			atomic.AddInt64(&r.routingErrors, 1)
			r.writeHTTPError(conn, 502, "Bad Gateway - TCP tunnel establishment timeout")
			return
		}
	}

	// Proxy through TCP tunnel with connection health validation
	if err := r.tcpTunnel.ProxyWebSocketConnectionWithRetry(domain, conn, httpReq); err != nil {
		// If connection fails (broken pipe), trigger demand-based establishment
		if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection") {
			r.logger.Warn("[HYBRID→TCP] TCP tunnel connection failed (%v), requesting new establishment...", err)

			// Remove dead connection and request new one
			r.tcpTunnel.RemoveDeadConnection(domain)

			if r.waitForTCPTunnelEstablishment(domain, conn, requestData, requestBody, clientIP, httpReq) {
				r.logger.Info("[HYBRID→TCP] TCP tunnel re-established successfully for domain: %s", domain)
				// Retry the WebSocket connection
				r.tcpTunnel.ProxyWebSocketConnection(domain, conn, httpReq)
			} else {
				r.logger.Error("[HYBRID→TCP] Failed to re-establish TCP tunnel for domain: %s", domain)
				atomic.AddInt64(&r.routingErrors, 1)
				r.writeHTTPError(conn, 502, "Bad Gateway - TCP tunnel re-establishment failed")
			}
		} else {
			r.logger.Error("[HYBRID→TCP] WebSocket proxy error: %v", err)
			atomic.AddInt64(&r.routingErrors, 1)
			r.writeHTTPError(conn, 500, "Internal Server Error - WebSocket proxy failed")
		}
	}

	r.logger.Debug("[HYBRID→TCP] WebSocket proxy completed")
}

// analyzeRequest analyzes the request to determine routing strategy
func (r *HybridTunnelRouter) analyzeRequest(requestData []byte) (isWebSocket bool, method string, path string) {
	requestStr := string(requestData)
	lines := strings.Split(requestStr, "\r\n")

	if len(lines) == 0 {
		return false, "", ""
	}

	// Parse request line (METHOD PATH HTTP/1.1)
	requestLine := lines[0]
	parts := strings.Fields(requestLine)
	if len(parts) >= 2 {
		method = parts[0]
		path = parts[1]
	}

	// Check for WebSocket upgrade headers
	requestLower := strings.ToLower(requestStr)
	isWebSocket = strings.Contains(requestLower, "upgrade: websocket") ||
		strings.Contains(requestLower, "connection: upgrade")

	// Force routing based on configuration - TCP paths take priority over gRPC paths
	// Check TCP paths first (more specific WebSocket patterns)
	for _, forcePath := range r.config.ForceTCPPaths {
		if strings.Contains(path, forcePath) {
			isWebSocket = true
			r.logger.Debug("[HYBRID] Path %s matched ForceTCPPaths pattern: %s", path, forcePath)
			return isWebSocket, method, path
		}
	}

	// Check for large files that should use gRPC chunked streaming (downloads)
	if r.isLargeFile(path) {
		isWebSocket = true // Mark as special handling to divert from normal gRPC path
		r.logger.Debug("[HYBRID] Path %s detected as large file, routing to gRPC chunked streaming", path)
		return isWebSocket, method, path
	}

	// Then check gRPC paths (only if not already matched by TCP or large file)
	for _, forcePath := range r.config.ForceGRPCPaths {
		if strings.Contains(path, forcePath) {
			isWebSocket = false
			r.logger.Debug("[HYBRID] Path %s matched ForceGRPCPaths pattern: %s", path, forcePath)
			break
		}
	}

	return isWebSocket, method, path
}

// isLargeFile determines if a file path is likely to be a large file that should use TCP streaming
func (r *HybridTunnelRouter) isLargeFile(path string) bool {
	pathLower := strings.ToLower(path)

	// Check for large file extensions
	for _, ext := range r.config.LargeFileExtensions {
		if strings.HasSuffix(pathLower, strings.ToLower(ext)) {
			return true
		}
	}

	// Check for large file paths
	for _, largePath := range r.config.LargeFilePaths {
		if strings.Contains(pathLower, strings.ToLower(largePath)) {
			return true
		}
	}

	return false
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request
func (r *HybridTunnelRouter) isWebSocketUpgrade(requestData []byte) bool {
	requestLower := strings.ToLower(string(requestData))
	return strings.Contains(requestLower, "upgrade: websocket") ||
		strings.Contains(requestLower, "connection: upgrade")
}

// routeToGRPCChunkedStreaming routes large files to gRPC chunked streaming for unlimited concurrency
func (r *HybridTunnelRouter) routeToGRPCChunkedStreaming(domain string, conn net.Conn, requestData []byte, requestBody io.Reader, clientIP, method, path string) {
	atomic.AddInt64(&r.grpcRequests, 1)

	r.logger.Debug("[HYBRID→gRPC-CHUNKED] 🚀 Routing large file via gRPC chunked streaming: %s %s", method, path)

	// Check if gRPC tunnel is available
	if !r.grpcTunnel.IsTunnelActive(domain) {
		r.logger.Error("[HYBRID→gRPC-CHUNKED] No active gRPC tunnel for domain: %s", domain)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 502, "Bad Gateway - gRPC tunnel not available for large file")
		return
	}

	// Parse HTTP request
	httpReq, err := r.parseHTTPRequest(requestData, requestBody)
	if err != nil {
		r.logger.Error("[HYBRID→gRPC-CHUNKED] Failed to parse large file request: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 400, "Bad Request - Invalid HTTP request")
		return
	}

	// Use the enhanced gRPC proxy with chunking support
	response, err := r.grpcTunnel.ProxyHTTPRequestWithChunking(domain, httpReq, clientIP)
	if err != nil {
		r.logger.Error("[HYBRID→gRPC-CHUNKED] gRPC chunked proxy error: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		if isTimeoutError(err) {
			atomic.AddInt64(&r.timeoutErrors, 1)
		}
		r.writeHTTPError(conn, 502, fmt.Sprintf("Bad Gateway - %v", err))
		return
	}

	// Write response back to client
	writer := bufio.NewWriter(conn)
	if err := response.Write(writer); err != nil {
		r.logger.Error("[HYBRID→gRPC-CHUNKED] Error writing response: %v", err)
		return
	}

	if err := writer.Flush(); err != nil {
		r.logger.Error("[HYBRID→gRPC-CHUNKED] Error flushing response: %v", err)
		return
	}

	r.logger.Debug("[HYBRID→gRPC-CHUNKED] ✅ Large file streaming completed via gRPC")
}

// parseHTTPRequest parses raw HTTP request data into http.Request
func (r *HybridTunnelRouter) parseHTTPRequest(requestData []byte, requestBody io.Reader) (*http.Request, error) {
	// Create a reader for the request
	requestReader := bufio.NewReader(strings.NewReader(string(requestData)))

	// Parse the request
	httpReq, err := http.ReadRequest(requestReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTP request: %w", err)
	}

	// If there's additional body data, set it
	if requestBody != nil {
		httpReq.Body = io.NopCloser(requestBody)
	}

	return httpReq, nil
}

// extractClientIP extracts client IP from connection
func (r *HybridTunnelRouter) extractClientIP(conn net.Conn) string {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		return tcpConn.RemoteAddr().(*net.TCPAddr).IP.String()
	}
	return conn.RemoteAddr().String()
}

// writeHTTPError writes an HTTP error response
func (r *HybridTunnelRouter) writeHTTPError(conn net.Conn, statusCode int, message string) {
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "Unknown Error"
	}

	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"X-Tunnel-Router: hybrid\r\n"+
		"\r\n"+
		"%s",
		statusCode, statusText, len(message), message)

	conn.Write([]byte(response))
}

// reportMetrics reports performance metrics
func (r *HybridTunnelRouter) reportMetrics() {
	ticker := time.NewTicker(r.config.MetricsInterval)
	defer ticker.Stop()

	for range ticker.C {
		total := atomic.LoadInt64(&r.totalRequests)
		grpc := atomic.LoadInt64(&r.grpcRequests)
		tcp := atomic.LoadInt64(&r.tcpRequests)
		ws := atomic.LoadInt64(&r.websocketUpgrades)
		errors := atomic.LoadInt64(&r.routingErrors)
		timeoutErrors := atomic.LoadInt64(&r.timeoutErrors)

		grpcPercent := float64(0)
		tcpPercent := float64(0)
		if total > 0 {
			grpcPercent = float64(grpc) / float64(total) * 100
			tcpPercent = float64(tcp) / float64(total) * 100
		}

		r.logger.Info("[HYBRID METRICS] Total: %d, gRPC: %d (%.1f%%), TCP: %d (%.1f%%), WebSocket: %d, Errors: %d (Timeout: %d)",
			total, grpc, grpcPercent, tcp, tcpPercent, ws, errors, timeoutErrors)
	}
}

// GetMetrics returns current routing metrics
func (r *HybridTunnelRouter) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_requests":     atomic.LoadInt64(&r.totalRequests),
		"grpc_requests":      atomic.LoadInt64(&r.grpcRequests),
		"tcp_requests":       atomic.LoadInt64(&r.tcpRequests),
		"websocket_upgrades": atomic.LoadInt64(&r.websocketUpgrades),
		"routing_errors":     atomic.LoadInt64(&r.routingErrors),
		"timeout_errors":     atomic.LoadInt64(&r.timeoutErrors),
	}
}

// IsTunnelDomain checks if any tunnel (gRPC or TCP) is active for the domain
func (r *HybridTunnelRouter) IsTunnelDomain(domain string) bool {
	return r.grpcTunnel.IsTunnelActive(domain) || r.tcpTunnel.IsTunnelDomain(domain)
}

// HasWebSocketConnection checks if TCP tunnel has WebSocket capability
func (r *HybridTunnelRouter) HasWebSocketConnection(domain string) bool {
	return r.tcpTunnel.HasWebSocketConnection(domain)
}

// waitForTCPTunnelEstablishment waits for TCP tunnel to be established for WebSocket requests
func (r *HybridTunnelRouter) waitForTCPTunnelEstablishment(domain string, conn net.Conn, requestData []byte, requestBody io.Reader, clientIP string, httpReq *http.Request) bool {
	requestID := fmt.Sprintf("ws-req-%d", time.Now().UnixNano())

	// Create pending connection
	pending := &PendingWebSocketConnection{
		Domain:      domain,
		Conn:        conn,
		RequestData: requestData,
		RequestBody: requestBody,
		ClientIP:    clientIP,
		HTTPReq:     httpReq,
		RequestID:   requestID,
		StartTime:   time.Now(),
		DoneChan:    make(chan bool, 1),
	}

	// Add to pending connections
	r.pendingConnectionsMu.Lock()
	if r.pendingConnections[domain] == nil {
		r.pendingConnections[domain] = make([]*PendingWebSocketConnection, 0)
	}
	r.pendingConnections[domain] = append(r.pendingConnections[domain], pending)
	r.pendingConnectionsMu.Unlock()

	// Signal client via gRPC to establish TCP tunnel
	go r.requestTCPTunnelEstablishment(domain, requestID)

	// Wait for tunnel establishment or timeout
	timeout := time.NewTimer(r.tunnelEstablishTimeout)
	defer timeout.Stop()

	select {
	case success := <-pending.DoneChan:
		// Remove from pending connections
		r.removePendingConnection(domain, requestID)
		return success
	case <-timeout.C:
		r.logger.Warn("[HYBRID→TCP] Timeout waiting for TCP tunnel establishment for domain: %s", domain)
		// Remove from pending connections
		r.removePendingConnection(domain, requestID)
		return false
	}
}

// requestTCPTunnelEstablishment signals the client via gRPC to establish a TCP tunnel
func (r *HybridTunnelRouter) requestTCPTunnelEstablishment(domain string, requestID string) {
	r.logger.Info("[HYBRID→TCP] Requesting TCP tunnel establishment for domain: %s (request: %s)", domain, requestID)

	// Create tunnel establishment request
	establishReq := &proto.TunnelEstablishRequest{
		TunnelType: proto.TunnelType_TUNNEL_TYPE_TCP,
		Domain:     domain,
		RequestId:  requestID,
		TimeoutMs:  int64(r.tunnelEstablishTimeout.Milliseconds()),
		Reason:     "WebSocket upgrade request pending",
	}

	// Send signal via gRPC tunnel
	if r.grpcTunnel.IsTunnelActive(domain) {
		err := r.grpcTunnel.SendTunnelEstablishRequest(domain, establishReq)
		if err != nil {
			r.logger.Error("[HYBRID→TCP] Failed to send tunnel establishment request: %v", err)
		} else {
			r.logger.Info("[HYBRID→TCP] TCP tunnel establishment request sent successfully")
		}
	} else {
		r.logger.Error("[HYBRID→TCP] No active gRPC tunnel to send establishment request for domain: %s", domain)
	}
}

// removePendingConnection removes a pending connection from the map
func (r *HybridTunnelRouter) removePendingConnection(domain string, requestID string) {
	r.pendingConnectionsMu.Lock()
	defer r.pendingConnectionsMu.Unlock()

	connections := r.pendingConnections[domain]
	for i, conn := range connections {
		if conn.RequestID == requestID {
			// Remove this connection
			r.pendingConnections[domain] = append(connections[:i], connections[i+1:]...)
			break
		}
	}

	// Clean up empty domain entries
	if len(r.pendingConnections[domain]) == 0 {
		delete(r.pendingConnections, domain)
	}
}

// OnTCPTunnelEstablished is called when a TCP tunnel is established to wake up pending connections
func (r *HybridTunnelRouter) OnTCPTunnelEstablished(domain string) {
	r.pendingConnectionsMu.Lock()
	defer r.pendingConnectionsMu.Unlock()

	if connections := r.pendingConnections[domain]; connections != nil {
		r.logger.Info("[HYBRID→TCP] TCP tunnel established for domain: %s, waking up %d pending connections", domain, len(connections))

		// Wake up all pending connections for this domain
		for _, conn := range connections {
			select {
			case conn.DoneChan <- true:
				r.logger.Debug("[HYBRID→TCP] Notified pending connection %s", conn.RequestID)
			default:
				// Channel might be closed or full, skip
			}
		}
	}
}
