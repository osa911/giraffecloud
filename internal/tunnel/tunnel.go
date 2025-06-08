package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ConnectionState represents the current state of the tunnel connection
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateMaintenance
	StateFailed
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateReconnecting:
		return "Reconnecting"
	case StateMaintenance:
		return "Maintenance"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	JitterEnabled   bool          `json:"jitter_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      -1, // Infinite retries
		InitialDelay:    1 * time.Second,
		MaxDelay:        5 * time.Minute,
		BackoffFactor:   2.0,
		JitterEnabled:   true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Tunnel represents a secure tunnel connection with enhanced reliability
type Tunnel struct {
	conn         net.Conn
	stopChan     chan struct{}
	token        string
	domain       string
	localPort    int
	logger       *logging.Logger

	// Enhanced connection management
	state        ConnectionState
	stateMutex   sync.RWMutex
	retryConfig  *RetryConfig
	retryCount   int
	lastError    error

	// Health monitoring
	healthTicker *time.Ticker
	lastPing     time.Time

	// Graceful shutdown
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewTunnel creates a new tunnel instance with enhanced features
func NewTunnel() *Tunnel {
	return &Tunnel{
		stopChan:    make(chan struct{}),
		logger:      logging.GetGlobalLogger(),
		state:       StateDisconnected,
		retryConfig: DefaultRetryConfig(),
	}
}

// SetRetryConfig allows customization of retry behavior
func (t *Tunnel) SetRetryConfig(config *RetryConfig) {
	t.retryConfig = config
}

// GetState returns the current connection state
func (t *Tunnel) GetState() ConnectionState {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()
	return t.state
}

// setState updates the connection state
func (t *Tunnel) setState(state ConnectionState) {
	t.stateMutex.Lock()
	defer t.stateMutex.Unlock()
	if t.state != state {
		t.logger.Info("Connection state changed: %s -> %s", t.state, state)
		t.state = state
	}
}

// Connect establishes a tunnel connection with retry logic
func (t *Tunnel) Connect(ctx context.Context, serverAddr, token, domain string, localPort int, tlsConfig *tls.Config) error {
	// Use the provided context instead of creating our own
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.token = token
	t.domain = domain
	t.localPort = localPort

	// Start the connection with retry logic
	return t.connectWithRetry(serverAddr, tlsConfig)
}

// connectWithRetry implements exponential backoff retry logic
func (t *Tunnel) connectWithRetry(serverAddr string, tlsConfig *tls.Config) error {
	t.retryCount = 0
	delay := t.retryConfig.InitialDelay

	for {
		select {
		case <-t.ctx.Done():
			return fmt.Errorf("connection cancelled")
		default:
		}

		// Check if we've exceeded max retries (if set)
		if t.retryConfig.MaxRetries > 0 && t.retryCount >= t.retryConfig.MaxRetries {
			t.setState(StateFailed)
			return fmt.Errorf("max retries (%d) exceeded, last error: %w", t.retryConfig.MaxRetries, t.lastError)
		}

		// Set appropriate state
		if t.retryCount == 0 {
			t.setState(StateConnecting)
		} else {
			t.setState(StateReconnecting)
			t.logger.Info("Retrying connection (attempt %d) in %v...", t.retryCount+1, delay)

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
			case <-t.ctx.Done():
				return fmt.Errorf("connection cancelled during retry")
			}
		}

		// Attempt connection
		err := t.attemptConnection(serverAddr, tlsConfig)
		if err == nil {
			// Success! Reset retry count and start health monitoring
			t.retryCount = 0
			t.setState(StateConnected)
			t.startHealthMonitoring()
			return nil
		}

		// Connection failed
		t.lastError = err
		t.retryCount++

		// Check if this is a maintenance mode error
		if isMaintenanceError(err) {
			t.setState(StateMaintenance)
			t.logger.Info("Server is in maintenance mode, will retry when available")
			delay = t.retryConfig.HealthCheckInterval // Use health check interval for maintenance
		} else {
			// Calculate next delay with exponential backoff
			delay = t.calculateNextDelay(delay)
			t.logger.Error("Connection failed (attempt %d): %v", t.retryCount, err)
		}
	}
}

// attemptConnection tries to establish a single connection
func (t *Tunnel) attemptConnection(serverAddr string, tlsConfig *tls.Config) error {
	// Simplify TLS config - use defaults for better compatibility
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true, // Only for development
		}
	}

	// Connect to server with TLS and timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Perform handshake with timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	resp, err := Perform(conn, t.token)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}
	conn.SetDeadline(time.Time{}) // Clear deadline

	// Update local values with server response
	t.domain = resp.Domain
	if t.localPort <= 0 {
		t.localPort = resp.TargetPort
	}

	// Check if the local port is actually listening
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		conn.Close()
		return fmt.Errorf("no service found listening on port %d - make sure your service is running first", t.localPort)
	}
	localConn.Close()

	// Store connection and start handling
	t.conn = conn
	t.lastPing = time.Now()

	t.logger.Info("Tunnel connected successfully. Domain: %s, Local Port: %d", t.domain, t.localPort)

	// Start HTTP forwarding
	t.wg.Add(1)
	go t.handleConnection()

	return nil
}

// calculateNextDelay implements exponential backoff with jitter
func (t *Tunnel) calculateNextDelay(currentDelay time.Duration) time.Duration {
	// Exponential backoff
	nextDelay := time.Duration(float64(currentDelay) * t.retryConfig.BackoffFactor)

	// Cap at max delay
	if nextDelay > t.retryConfig.MaxDelay {
		nextDelay = t.retryConfig.MaxDelay
	}

	// Add jitter to prevent thundering herd
	if t.retryConfig.JitterEnabled {
		jitter := time.Duration(float64(nextDelay) * 0.1 * (2*rand.Float64() - 1))
		nextDelay += jitter
	}

	return nextDelay
}

// isMaintenanceError checks if the error indicates server maintenance
func isMaintenanceError(err error) bool {
	// Check for specific maintenance-related error messages
	errStr := err.Error()
	maintenanceKeywords := []string{
		"maintenance",
		"service unavailable",
		"temporarily unavailable",
		"503",
	}

	for _, keyword := range maintenanceKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

// contains is a simple string contains check (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    (len(s) > len(substr) &&
		     (s[:len(substr)] == substr ||
		      s[len(s)-len(substr):] == substr ||
		      indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// startHealthMonitoring starts the health monitoring goroutine
func (t *Tunnel) startHealthMonitoring() {
	t.logger.Info("Health monitoring disabled - relying on HTTP traffic for connection health")
}

// reconnect handles reconnection logic
func (t *Tunnel) reconnect() {
	// Close current connection
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}

	// Stop health monitoring
	if t.healthTicker != nil {
		t.healthTicker.Stop()
		t.healthTicker = nil
	}

	// Start reconnection process
	go func() {
		serverAddr := fmt.Sprintf("%s:%d", "tunnel.giraffecloud.xyz", 4443) // TODO: make configurable
		t.connectWithRetry(serverAddr, nil)
	}()
}

// handleConnection handles HTTP requests from the tunnel server
func (t *Tunnel) handleConnection() {
	defer t.wg.Done()
	defer func() {
		if t.conn != nil {
			t.conn.Close()
		}
		t.logger.Info("Tunnel connection closed")
	}()

	t.logger.Info("Starting HTTP forwarding for tunnel connection")

	// Create buffered reader for parsing HTTP
	tunnelReader := bufio.NewReader(t.conn)

	// Handle incoming HTTP requests from the tunnel
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Set read timeout for incoming requests
			t.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Parse the HTTP request using Go's built-in HTTP parser
			request, err := http.ReadRequest(tunnelReader)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is normal, continue
					continue
				}
				// Connection closed or error - trigger reconnection
				t.logger.Info("Tunnel connection closed: %v", err)
				t.reconnect()
				return
			}

			// We have an HTTP request, reset deadline
			t.conn.SetReadDeadline(time.Time{})

			t.logger.Info("Received HTTP request: %s %s", request.Method, request.URL.Path)

			// Check if this is a WebSocket upgrade request
			isWebSocket := t.isWebSocketUpgrade(request)
			if isWebSocket {
				t.logger.Info("[WEBSOCKET DEBUG] Detected WebSocket upgrade request")
				t.handleWebSocketUpgrade(request)
				return // WebSocket connections don't continue the HTTP loop
			}

			// Handle regular HTTP request
			t.handleHTTPRequest(request)
		}
	}
}

// isWebSocketUpgrade checks if the HTTP request is a WebSocket upgrade request
func (t *Tunnel) isWebSocketUpgrade(r *http.Request) bool {
	connection := r.Header.Get("Connection")
	upgrade := r.Header.Get("Upgrade")
	webSocketKey := r.Header.Get("Sec-WebSocket-Key")

	// Connection header should contain "upgrade" (case-insensitive)
	connectionUpgrade := false
	for _, part := range strings.Split(strings.ToLower(connection), ",") {
		if strings.TrimSpace(part) == "upgrade" {
			connectionUpgrade = true
			break
		}
	}

	// Upgrade header should be "websocket" (case-insensitive)
	upgradeWebSocket := strings.ToLower(strings.TrimSpace(upgrade)) == "websocket"

	// Must have Sec-WebSocket-Key header
	hasWebSocketKey := webSocketKey != ""

	return connectionUpgrade && upgradeWebSocket && hasWebSocketKey
}

// handleWebSocketUpgrade handles the WebSocket upgrade process
func (t *Tunnel) handleWebSocketUpgrade(request *http.Request) {
	t.logger.Info("[WEBSOCKET DEBUG] Handling WebSocket upgrade to local service on port %d", t.localPort)

	// Connect to local service for WebSocket upgrade
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Failed to connect to local service: %v", err)
		// Send error response back through tunnel
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		t.conn.Write([]byte(errorResponse))
		return
	}

	// Forward the WebSocket upgrade request to local service
	if err := request.Write(localConn); err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Failed to write upgrade request to local service: %v", err)
		localConn.Close()
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		t.conn.Write([]byte(errorResponse))
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] Upgrade request forwarded to local service, reading response")

	// Read the upgrade response from local service
	localReader := bufio.NewReader(localConn)
	response, err := http.ReadResponse(localReader, request)
	if err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Error reading upgrade response from local service: %v", err)
		localConn.Close()
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		t.conn.Write([]byte(errorResponse))
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] Received upgrade response: %s", response.Status)

	// Write the upgrade response back to tunnel
	if err := response.Write(t.conn); err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade response to tunnel: %v", err)
		localConn.Close()
		return
	}

	// Check if the upgrade was successful (101 Switching Protocols)
	if response.StatusCode != 101 {
		t.logger.Error("[WEBSOCKET DEBUG] WebSocket upgrade failed with status: %d", response.StatusCode)
		localConn.Close()
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] WebSocket upgrade successful, starting bidirectional forwarding")

	// Start bidirectional copying between tunnel and local service
	errChan := make(chan error, 2)

	// Copy from tunnel to local service
	go func() {
		_, err := io.Copy(localConn, t.conn)
		errChan <- err
	}()

	// Copy from local service to tunnel
	go func() {
		_, err := io.Copy(t.conn, localConn)
		errChan <- err
	}()

	// Wait for either direction to close or error
	err = <-errChan
	if err != nil {
		t.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed: %v", err)
	} else {
		t.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed normally")
	}

	localConn.Close()
	t.logger.Info("[WEBSOCKET DEBUG] WebSocket forwarding completed")
}

// handleHTTPRequest handles regular HTTP requests (non-WebSocket)
func (t *Tunnel) handleHTTPRequest(request *http.Request) {
	// Connect to local service for this request
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		t.logger.Error("Failed to connect to local service: %v", err)
		// Send error response back through tunnel
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		t.conn.Write([]byte(errorResponse))
		return
	}

	// Forward the request to local service
	if err := request.Write(localConn); err != nil {
		t.logger.Error("Failed to write request to local service: %v", err)
		localConn.Close()
		return
	}

	t.logger.Info("Request forwarded to local service, reading response")

	// Read response from local service
	localReader := bufio.NewReader(localConn)
	response, err := http.ReadResponse(localReader, request)
	if err != nil {
		t.logger.Error("Error reading response from local service: %v", err)
		localConn.Close()
		return
	}

	// Write response back to tunnel
	if err := response.Write(t.conn); err != nil {
		t.logger.Error("Error writing response to tunnel: %v", err)
		localConn.Close()
		return
	}

	localConn.Close()
	t.logger.Info("HTTP request/response cycle completed")
}

// Disconnect closes the tunnel connection and cleans up resources
func (t *Tunnel) Disconnect() error {
	// Cancel context to stop all goroutines
	t.cancel()

	// Stop health monitoring
	if t.healthTicker != nil {
		t.healthTicker.Stop()
		t.healthTicker = nil
	}

	// Close stop channel
	select {
	case <-t.stopChan:
	default:
		close(t.stopChan)
	}

	// Close connection if exists
	var err error
	if t.conn != nil {
		// Set a deadline for graceful shutdown
		t.conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Don't send JSON control messages over HTTP connection
		// as they interfere with HTTP traffic

		// Close the connection
		err = t.conn.Close()
		t.conn = nil
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-time.After(10 * time.Second):
		t.logger.Info("Timeout waiting for goroutines to finish")
	}

	t.setState(StateDisconnected)
	t.logger.Info("Tunnel disconnected")
	return err
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.GetState() == StateConnected
}

// GetStats returns connection statistics
func (t *Tunnel) GetStats() map[string]interface{} {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()

	stats := map[string]interface{}{
		"state":        t.state.String(),
		"retry_count":  t.retryCount,
		"domain":       t.domain,
		"local_port":   t.localPort,
		"last_ping":    t.lastPing,
	}

	if t.lastError != nil {
		stats["last_error"] = t.lastError.Error()
	}

	return stats
}