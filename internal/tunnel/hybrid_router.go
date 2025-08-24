package tunnel

import (
	"bufio"
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

	// Configuration
	config *HybridRouterConfig

	// Usage aggregation
	usage UsageRecorder
	// Quotas
	quota QuotaChecker
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
		logger: logging.GetGlobalLogger(),
		config: config,
	}

	// Create gRPC tunnel server (for HTTP traffic)
	grpcConfig := DefaultGRPCTunnelConfig()
	router.grpcTunnel = NewGRPCTunnelServer(tokenRepo, tunnelRepo, tunnelService, grpcConfig)

	// Create TCP tunnel server (for WebSocket traffic)
	router.tcpTunnel = NewServer(tokenRepo, tunnelRepo, tunnelService)

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

	// Check if TCP tunnel is available
	if !r.tcpTunnel.IsTunnelDomain(domain) {
		r.logger.Error("[HYBRID→TCP] No active TCP tunnel for domain: %s", domain)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 502, "Bad Gateway - TCP tunnel not available")
		return
	}

	// Parse HTTP request for WebSocket upgrade
	httpReq, err := r.parseHTTPRequest(requestData, requestBody)
	if err != nil {
		r.logger.Error("[HYBRID→TCP] Failed to parse WebSocket request: %v", err)
		atomic.AddInt64(&r.routingErrors, 1)
		r.writeHTTPError(conn, 400, "Bad Request - Invalid WebSocket request")
		return
	}

	// Proxy through TCP tunnel (existing WebSocket handling)
	r.tcpTunnel.ProxyWebSocketConnection(domain, conn, httpReq)

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

		grpcPercent := float64(0)
		tcpPercent := float64(0)
		if total > 0 {
			grpcPercent = float64(grpc) / float64(total) * 100
			tcpPercent = float64(tcp) / float64(total) * 100
		}

		r.logger.Info("[HYBRID METRICS] Total: %d, gRPC: %d (%.1f%%), TCP: %d (%.1f%%), WebSocket: %d, Errors: %d",
			total, grpc, grpcPercent, tcp, tcpPercent, ws, errors)
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
