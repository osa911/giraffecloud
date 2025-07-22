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
	grpcTunnel *GRPCTunnelServer  // Handles HTTP traffic with unlimited concurrency
	tcpTunnel  *TunnelServer      // Handles WebSocket traffic (legacy)
	logger     *logging.Logger

	// Performance metrics
	totalRequests      int64
	grpcRequests       int64
	tcpRequests        int64
	websocketUpgrades  int64
	routingErrors      int64

	// Configuration
	config *HybridRouterConfig
}

// HybridRouterConfig holds configuration for the hybrid router
type HybridRouterConfig struct {
	// Server addresses
	GRPCAddress string
	TCPAddress  string

	// Request classification
	ForceGRPCPaths    []string // Paths that must use gRPC
	ForceTCPPaths     []string // Paths that must use TCP

	// Performance settings
	EnableMetrics     bool
	MetricsInterval   time.Duration

	// Security settings
	EnableRateLimit   bool
	MaxRequestsPerMin int
}

// DefaultHybridRouterConfig returns production-ready configuration
func DefaultHybridRouterConfig() *HybridRouterConfig {
	return &HybridRouterConfig{
		GRPCAddress:       ":4444", // Different port for gRPC
		TCPAddress:        ":4443", // Original port for TCP/WebSocket
		ForceGRPCPaths:    []string{"/api/", "/assets/", "/media/", "/static/"},
		ForceTCPPaths:     []string{"/ws/", "/websocket/", "/socket.io/"},
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
	isWebSocket, httpMethod, requestPath := r.analyzeRequest(requestData)

	r.logger.Debug("[HYBRID] Request from %s: %s %s (WebSocket: %t)",
		clientIP, httpMethod, requestPath, isWebSocket)

	// Route based on request type
	if isWebSocket {
		r.routeToTCPTunnel(domain, conn, requestData, requestBody, clientIP)
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
	response, err := r.grpcTunnel.ProxyHTTPRequest(domain, httpReq, clientIP)
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

	// Force routing based on configuration
	for _, forcePath := range r.config.ForceTCPPaths {
		if strings.Contains(path, forcePath) {
			isWebSocket = true
			break
		}
	}

	for _, forcePath := range r.config.ForceGRPCPaths {
		if strings.Contains(path, forcePath) {
			isWebSocket = false
			break
		}
	}

	return isWebSocket, method, path
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