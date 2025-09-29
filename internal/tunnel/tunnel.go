package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
	MaxRetries          int           `json:"max_retries"`
	InitialDelay        time.Duration `json:"initial_delay"`
	MaxDelay            time.Duration `json:"max_delay"`
	BackoffFactor       float64       `json:"backoff_factor"`
	JitterEnabled       bool          `json:"jitter_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:          -1, // Infinite retries
		InitialDelay:        1 * time.Second,
		MaxDelay:            5 * time.Minute,
		BackoffFactor:       2.0,
		JitterEnabled:       true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Tunnel represents a secure tunnel connection with enhanced reliability
type Tunnel struct {
	conn      net.Conn
	stopChan  chan struct{}
	token     string
	domain    string
	localPort int
	logger    *logging.Logger

	// Singleton management
	singletonManager *SingletonManager

	// Enhanced connection management
	state       ConnectionState
	stateMutex  sync.RWMutex
	retryConfig *RetryConfig
	retryCount  int
	lastError   error

	// Health monitoring
	healthTicker *time.Ticker
	lastPing     time.Time

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// WebSocket connection
	wsConn net.Conn

	// HTTP connections for large file streaming in hybrid mode
	httpConnections []net.Conn

	// Reconnection coordination
	reconnectMutex         sync.Mutex
	isReconnecting         bool
	isIntentionalReconnect bool // Flag to prevent race conditions during WebSocket recycling

	// WebSocket re-establishment loop control (hybrid mode)
	wsReconnectMu         sync.Mutex
	wsReconnectInProgress bool

	// Streaming configuration
	streamConfig *StreamingConfig

	// PRODUCTION-GRADE: gRPC Tunnel Client for unlimited HTTP concurrency
	grpcClient  *GRPCTunnelClient
	grpcEnabled bool

	// Hook invoked each time a connection is successfully established (including reconnects)
	onConnectHook func()
}

// TunnelState represents preserved tunnel state for connection restoration
type TunnelState struct {
	Token      string          `json:"token"`
	Domain     string          `json:"domain"`
	LocalPort  int             `json:"local_port"`
	ServerAddr string          `json:"server_addr"`
	TLSConfig  *tls.Config     `json:"-"` // Can't serialize, will need to recreate
	State      ConnectionState `json:"state"`
	Timestamp  time.Time       `json:"timestamp"`

	// Connection details
	GRPCEnabled bool `json:"grpc_enabled"`

	// Configuration
	RetryConfig  *RetryConfig     `json:"retry_config"`
	StreamConfig *StreamingConfig `json:"stream_config"`
}

// NewTunnel creates a new tunnel instance with enhanced features
func NewTunnel() *Tunnel {
	singletonManager, err := NewSingletonManager()
	if err != nil {
		// Log error but don't fail - singleton is optional for backward compatibility
		logging.GetGlobalLogger().Warn("Failed to create singleton manager: %v", err)
	}

	return &Tunnel{
		stopChan:         make(chan struct{}),
		logger:           logging.GetGlobalLogger(),
		singletonManager: singletonManager,
		state:            StateDisconnected,
		retryConfig:      DefaultRetryConfig(),
		streamConfig:     DefaultStreamingConfig(), // Use default streaming config
	}
}

// SetOnConnectHook registers a hook to be called after each successful connection establishment
func (t *Tunnel) SetOnConnectHook(hook func()) {
	t.onConnectHook = hook
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

// Connect establishes tunnel connections with retry logic
func (t *Tunnel) Connect(ctx context.Context, serverAddr, token, domain string, localPort int, tlsConfig *tls.Config) error {
	// Check singleton lock before connecting
	if t.singletonManager != nil {
		// Clean up any stale locks first
		if err := t.singletonManager.CleanupStaleLock(); err != nil {
			t.logger.Warn("Failed to cleanup stale lock: %v", err)
		}

		// Check for service conflicts
		if err := t.singletonManager.CheckServiceConflict(); err != nil {
			return fmt.Errorf("service conflict detected: %w", err)
		}

		// Acquire singleton lock
		if err := t.singletonManager.AcquireLock(); err != nil {
			return fmt.Errorf("failed to acquire singleton lock: %w", err)
		}
	}

	// Use the provided context instead of creating our own
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.token = token
	t.domain = domain
	t.localPort = localPort

	// Start the connections with retry logic
	return t.connectWithRetry(serverAddr, tlsConfig)
}

// connectWithRetry implements exponential backoff retry logic for both connection types
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

		// Attempt to establish both HTTP and WebSocket connections
		err := t.attemptDualConnections(serverAddr, tlsConfig)
		if err == nil {
			// Success! Reset retry count and start health monitoring
			t.retryCount = 0
			t.setState(StateConnected)
			t.startHealthMonitoring()
			if t.onConnectHook != nil {
				// Invoke hook safely in a separate goroutine
				go func() {
					defer func() { _ = recover() }()
					t.onConnectHook()
				}()
			}
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

// attemptDualConnections tries to establish both gRPC (HTTP) and TCP (WebSocket) tunnel connections
func (t *Tunnel) attemptDualConnections(serverAddr string, tlsConfig *tls.Config) error {
	// PRODUCTION-GRADE: Create secure TLS config with proper certificate validation
	if tlsConfig == nil {
		// Normalize config home when running elevated
		EnsureConsistentConfigHome()
		// PRODUCTION-GRADE: Load configuration and REQUIRE proper certificates
		cfg, err := LoadConfig()
		if err != nil {
			return fmt.Errorf("SECURITY ERROR: Failed to load config for certificates: %w", err)
		}

		// PRODUCTION-GRADE: Create secure TLS configuration with proper certificates
		tlsConfig, err = CreateSecureTLSConfig(cfg.Security.CACert, cfg.Security.ClientCert, cfg.Security.ClientKey)
		if err != nil {
			return fmt.Errorf("SECURITY ERROR: Failed to create secure TLS config: %w", err)
		}

		t.logger.Info("🔐 PRODUCTION-GRADE: Using secure TLS with certificate validation (InsecureSkipVerify: FALSE)")

		// CRITICAL: Force fresh TLS state during reconnection to prevent ERR_SSL_PROTOCOL_ERROR
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ClientSessionCache = nil                      // Completely disable session cache
		tlsConfig.SessionTicketsDisabled = true                 // Disable session tickets
		tlsConfig.Renegotiation = tls.RenegotiateNever          // Disable renegotiation
		tlsConfig.Time = func() time.Time { return time.Now() } // Force fresh time for each connection
	}

	t.logger.Info("🚀 Starting PRODUCTION-GRADE tunnel establishment...")

	// Step 1: Establish or reuse gRPC tunnel for HTTP traffic (unlimited concurrency)
	t.logger.Info("📡 Establishing gRPC tunnel for HTTP traffic...")

	// Parse server address for gRPC port
	grpcServerAddr := strings.Replace(serverAddr, ":4443", ":4444", 1) // Use gRPC port

	// Reuse existing client if available, otherwise create
	if t.grpcClient == nil {
		grpcConfig := DefaultGRPCClientConfig()
		t.grpcClient = NewGRPCTunnelClient(grpcServerAddr, t.domain, t.token, int32(t.localPort), grpcConfig)

		// Set up tunnel establishment handler for demand-based tunnel creation
		t.grpcClient.SetTunnelEstablishHandler(t.handleTunnelEstablishRequest)

		if err := t.grpcClient.Start(); err != nil {
			t.logger.Error("Failed to establish gRPC tunnel: %v", err)
			t.grpcEnabled = false
		} else {
			t.logger.Info("✅ gRPC tunnel established successfully - unlimited HTTP concurrency enabled! Client ID: %s", t.grpcClient.GetClientID())
			t.grpcEnabled = true
		}
	} else {
		// Make sure handler is set on existing client
		t.grpcClient.SetTunnelEstablishHandler(t.handleTunnelEstablishRequest)

		if t.grpcClient.IsConnected() {
			t.logger.Info("Reusing existing gRPC tunnel client (Client ID: %s)", t.grpcClient.GetClientID())
			t.grpcEnabled = true
		} else {
			t.logger.Info("Existing gRPC client not connected; attempting to start (Client ID: %s)", t.grpcClient.GetClientID())
			if err := t.grpcClient.Start(); err != nil {
				t.logger.Error("Failed to (re)start existing gRPC client: %v", err)
				t.grpcEnabled = false
			} else {
				t.logger.Info("✅ gRPC client (re)started successfully (Client ID: %s)", t.grpcClient.GetClientID())
				t.grpcEnabled = true
			}
		}
	}

	// Step 2: TCP tunnel is now DEMAND-BASED (established only when needed)
	if t.grpcEnabled {
		// In hybrid mode: gRPC handles ALL HTTP traffic (including large files via chunked streaming)
		// TCP tunnel will be established ON-DEMAND when WebSocket requests arrive
		t.logger.Info("🎯 HYBRID MODE ACTIVE: gRPC (ALL HTTP + Chunked Streaming) + TCP (ON-DEMAND for WebSockets)")
		t.logger.Info("🚀 PRODUCTION-GRADE: Unlimited concurrency + demand-based tunnel establishment!")
		t.logger.Info("⚡ TCP tunnel will be established automatically when WebSocket requests arrive")
		return nil
	}

	// gRPC tunnel establishment failed - this is a critical error in demand-based architecture
	return fmt.Errorf("gRPC tunnel is required for demand-based tunnel establishment - cannot proceed without it")
}

// startWebSocketReconnectLoop continuously retries establishing the WebSocket tunnel
// while gRPC remains connected. It exits on success or when the context is cancelled.
func (t *Tunnel) startWebSocketReconnectLoop(serverAddr string, tlsConfig *tls.Config) {
	// Avoid multiple concurrent loops
	t.wsReconnectMu.Lock()
	if t.wsReconnectInProgress || t.ctx == nil {
		// Already running, or no active context
		t.wsReconnectMu.Unlock()
		return
	}
	// If a WebSocket connection already exists, nothing to do
	if t.wsConn != nil {
		t.wsReconnectMu.Unlock()
		return
	}
	// Mark as in progress
	t.wsReconnectInProgress = true
	t.wsReconnectMu.Unlock()

	go func() {
		defer func() {
			// Reset flag on exit
			t.wsReconnectMu.Lock()
			t.wsReconnectInProgress = false
			t.wsReconnectMu.Unlock()
		}()

		// Backoff settings based on retryConfig
		delay := t.retryConfig.InitialDelay
		for {
			// Exit conditions
			select {
			case <-t.ctx.Done():
				return
			default:
			}

			// Do not interfere with a full reconnection in progress
			if t.isReconnecting {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// If gRPC is not enabled/connected anymore, stop this loop
			if !t.grpcEnabled || t.grpcClient == nil || !t.grpcClient.IsConnected() {
				return
			}

			// Prepare TLS config if needed to ensure fresh TLS state
			useTLS := tlsConfig
			if useTLS == nil {
				EnsureConsistentConfigHome()
				cfg, err := LoadConfig()
				if err != nil {
					// If config can't be loaded, retry later
					t.logger.Warn("[WS RETRY] Failed to load TLS config: %v", err)
					// Exponential backoff with jitter
					time.Sleep(t.calculateNextDelay(delay))
					continue
				}
				c, err := CreateSecureTLSConfig(cfg.Security.CACert, cfg.Security.ClientCert, cfg.Security.ClientKey)
				if err != nil {
					t.logger.Warn("[WS RETRY] Failed to create TLS config: %v", err)
					time.Sleep(t.calculateNextDelay(delay))
					continue
				}
				c = c.Clone()
				c.ClientSessionCache = nil
				c.SessionTicketsDisabled = true
				c.Renegotiation = tls.RenegotiateNever
				c.Time = func() time.Time { return time.Now() }
				useTLS = c
			}

			// Attempt to establish WebSocket connection
			wsConn, err := t.establishConnection(serverAddr, useTLS, "websocket")
			if err != nil {
				t.logger.Warn("[WS RETRY] WebSocket reconnect attempt failed: %v", err)
				// Exponential backoff with jitter
				delay = t.calculateNextDelay(delay)
				time.Sleep(delay)
				continue
			}

			// Success: set connection and start handler
			t.wsConn = wsConn
			t.logger.Info("✅ WebSocket tunnel re-established successfully")
			t.wg.Add(1)
			go t.handleWebSocketConnection(wsConn)
			return
		}
	}()
}

// establishConnection establishes a single tunnel connection of specified type
func (t *Tunnel) establishConnection(serverAddr string, tlsConfig *tls.Config, connType string) (net.Conn, error) {
	// Connect to server with TLS and timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", serverAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	// Perform handshake with timeout and connection type
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	resp, err := t.performHandshake(conn, t.token, connType)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %w", err)
	}
	conn.SetDeadline(time.Time{}) // Clear deadline

	// Update local values with server response (only on first successful connection)
	if t.domain == "" {
		t.domain = resp.Domain
	}
	if t.localPort <= 0 {
		t.localPort = resp.TargetPort
	}

	// Check if the local port is actually listening (only once)
	if connType == "http" {
		localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("no service found listening on port %d - make sure your service is running first", t.localPort)
		}
		localConn.Close()
	}

	t.logger.Info("%s tunnel connection established successfully", strings.Title(connType))
	return conn, nil
}

// performHandshake performs the handshake for a specific connection type
func (t *Tunnel) performHandshake(conn net.Conn, token, connType string) (*TunnelHandshakeResponse, error) {
	// Create JSON encoder/decoder
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// Send handshake request with connection type
	req := TunnelHandshakeRequest{
		Token:          token,
		ConnectionType: connType,
	}

	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	// Read handshake response
	var resp TunnelHandshakeResponse
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("handshake failed: %s", resp.Message)
	}

	return &resp, nil
}

// handleHTTPConnection handles HTTP requests from the tunnel server
func (t *Tunnel) handleHTTPConnection(conn net.Conn, connIndex ...int) {
	defer t.wg.Done()
	defer func() {
		if conn != nil {
			conn.Close()
		}
		// Log with connection index if provided
		if len(connIndex) > 0 {
			t.logger.Info("HTTP tunnel connection %d closed", connIndex[0])
		} else {
			t.logger.Info("HTTP tunnel connection closed")
		}
	}()

	// Log with connection index if provided
	if len(connIndex) > 0 {
		t.logger.Info("Starting HTTP forwarding for tunnel connection %d", connIndex[0])
	} else {
		t.logger.Info("Starting HTTP forwarding for tunnel connection")
	}

	// Create buffered reader for parsing HTTP
	tunnelReader := bufio.NewReader(conn)

	// Handle incoming HTTP requests from the tunnel
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Set read timeout for incoming requests
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Parse the HTTP request using Go's built-in HTTP parser
			request, err := http.ReadRequest(tunnelReader)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is normal, continue
					continue
				}
				// Connection closed or error - check if reconnection is needed
				t.logger.Info("HTTP tunnel connection closed: %v", err)
				if !isExpectedConnectionClose(err) {
					t.coordinatedReconnect()
				}
				return
			}

			// We have an HTTP request, reset deadline
			conn.SetReadDeadline(time.Time{})

			t.logger.Info("Received HTTP request: %s %s", request.Method, request.URL.Path)

			// Handle regular HTTP request
			t.handleHTTPRequest(request, conn)
		}
	}
}

// handleWebSocketConnection handles WebSocket traffic from the tunnel server
func (t *Tunnel) handleWebSocketConnection(conn net.Conn) {
	defer t.wg.Done()
	defer func() {
		if conn != nil {
			conn.Close()
		}
		t.logger.Info("WebSocket tunnel connection closed")
	}()

	t.logger.Info("Starting WebSocket forwarding for tunnel connection")

	// Create buffered reader for parsing HTTP
	tunnelReader := bufio.NewReader(conn)

	// Handle incoming WebSocket upgrade requests from the tunnel
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Set read timeout for incoming requests
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Parse the HTTP request using Go's built-in HTTP parser
			request, err := http.ReadRequest(tunnelReader)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is normal, continue
					continue
				}
				// Connection closed or error - check if reconnection is needed
				t.logger.Info("WebSocket tunnel connection closed: %v", err)
				if !isExpectedConnectionClose(err) {
					t.coordinatedReconnect()
				}
				return
			}

			// We have an HTTP request, reset deadline
			conn.SetReadDeadline(time.Time{})

			t.logger.Info("Received WebSocket upgrade request: %s %s", request.Method, request.URL.Path)

			// Handle WebSocket upgrade - this will consume the entire connection
			t.handleWebSocketUpgradeOnDedicatedConnection(request, conn)

			// After a WebSocket session completes, the tunnel connection is no longer usable
			// for HTTP parsing due to the bidirectional copying. We need to reconnect.
			t.logger.Info("[WEBSOCKET DEBUG] WebSocket session completed, triggering tunnel reconnection")
			t.coordinatedReconnectWithContext(true) // Mark as intentional
			return
		}
	}
}

// handleWebSocketUpgradeOnDedicatedConnection handles WebSocket upgrade on the dedicated WebSocket tunnel
func (t *Tunnel) handleWebSocketUpgradeOnDedicatedConnection(request *http.Request, tunnelConn net.Conn) {
	t.logger.Info("[WEBSOCKET DEBUG] Handling WebSocket upgrade to local service on port %d", t.localPort)

	// Connect to local service for WebSocket upgrade
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Failed to connect to local service: %v", err)
		// Send error response back through tunnel
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		tunnelConn.Write([]byte(errorResponse))
		return
	}
	defer localConn.Close()

	// Forward the WebSocket upgrade request to local service
	if err := request.Write(localConn); err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Failed to write upgrade request to local service: %v", err)
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		tunnelConn.Write([]byte(errorResponse))
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] Upgrade request forwarded to local service, reading response")

	// Read the upgrade response from local service
	localReader := bufio.NewReader(localConn)
	response, err := http.ReadResponse(localReader, request)
	if err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Error reading upgrade response from local service: %v", err)
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		tunnelConn.Write([]byte(errorResponse))
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] Received upgrade response: %s", response.Status)

	// Write the upgrade response back to tunnel
	if err := response.Write(tunnelConn); err != nil {
		t.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade response to tunnel: %v", err)
		return
	}

	// Check if the upgrade was successful (101 Switching Protocols)
	if response.StatusCode != 101 {
		t.logger.Error("[WEBSOCKET DEBUG] WebSocket upgrade failed with status: %d", response.StatusCode)
		return
	}

	t.logger.Info("[WEBSOCKET DEBUG] WebSocket upgrade successful, starting bidirectional forwarding")

	// Start bidirectional copying between tunnel and local service
	errChan := make(chan error, 2)

	// Copy from tunnel to local service
	go func() {
		_, err := io.Copy(localConn, tunnelConn)
		t.logger.Info("[WEBSOCKET DEBUG] Tunnel->Local copy finished: %v", err)
		errChan <- err
	}()

	// Copy from local service to tunnel
	go func() {
		_, err := io.Copy(tunnelConn, localConn)
		t.logger.Info("[WEBSOCKET DEBUG] Local->Tunnel copy finished: %v", err)
		errChan <- err
	}()

	// Wait for either direction to close or error
	err = <-errChan
	if err != nil {
		t.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed: %v", err)
	} else {
		t.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed normally")
	}

	t.logger.Info("[WEBSOCKET DEBUG] WebSocket forwarding completed - tunnel connection is now unusable")
}

// handleHTTPRequest handles regular HTTP requests (non-WebSocket) on the HTTP tunnel
func (t *Tunnel) handleHTTPRequest(request *http.Request, tunnelConn net.Conn) {
	// Check if this is a media request that needs optimized handling
	isMediaRequest := t.isMediaRequest(request)

	// Connect to local service for this request
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		t.logger.Error("Failed to connect to local service: %v", err)
		// Send error response back through tunnel
		errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
		tunnelConn.Write([]byte(errorResponse))
		return
	}
	defer localConn.Close()

	// Forward the request to local service
	if err := request.Write(localConn); err != nil {
		t.logger.Error("Failed to write request to local service: %v", err)
		return
	}

	if isMediaRequest {
		t.logger.Info("Handling media request with optimized streaming: %s %s", request.Method, request.URL.Path)
		t.handleMediaResponse(localConn, tunnelConn)
	} else {
		t.logger.Info("Request forwarded to local service, reading response")
		t.handleRegularResponse(request, localConn, tunnelConn)
	}
}

// isMediaRequest checks if this is a media/video request
func (t *Tunnel) isMediaRequest(request *http.Request) bool {
	if !t.streamConfig.EnableMediaOptimization {
		return false
	}

	path := request.URL.Path

	// Check for media file extensions from config
	for _, ext := range t.streamConfig.MediaExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	// Check for Range requests (common for video streaming)
	if request.Header.Get("Range") != "" {
		return true
	}

	// Check for media paths from config
	for _, mediaPath := range t.streamConfig.MediaPaths {
		if strings.Contains(path, mediaPath) {
			return true
		}
	}

	return false
}

// handleMediaResponse handles media responses with optimized streaming
func (t *Tunnel) handleMediaResponse(localConn, tunnelConn net.Conn) {
	t.logger.Info("Starting optimized media streaming")

	// Use larger buffers for media streaming
	buffer := make([]byte, t.streamConfig.MediaBufferSize)

	// Start bidirectional copying with optimized buffers
	errChan := make(chan error, 2)

	// Copy from local service to tunnel (response)
	go func() {
		_, err := io.CopyBuffer(tunnelConn, localConn, buffer)
		errChan <- err
	}()

	// Copy from tunnel to local service (for any additional data)
	go func() {
		buffer2 := make([]byte, t.streamConfig.MediaBufferSize)
		_, err := io.CopyBuffer(localConn, tunnelConn, buffer2)
		errChan <- err
	}()

	// Wait for either direction to complete
	err := <-errChan
	if err != nil && err != io.EOF {
		t.logger.Info("Media streaming completed with: %v", err)
	} else {
		t.logger.Info("Media streaming completed successfully")
	}
}

// handleRegularResponse handles regular HTTP responses
func (t *Tunnel) handleRegularResponse(request *http.Request, localConn, tunnelConn net.Conn) {
	// Read response from local service
	localReader := bufio.NewReader(localConn)
	response, err := http.ReadResponse(localReader, request)
	if err != nil {
		t.logger.Error("Error reading response from local service: %v", err)
		return
	}

	// Write response back to tunnel
	if err := response.Write(tunnelConn); err != nil {
		t.logger.Error("Error writing response to tunnel: %v", err)
		return
	}

	t.logger.Info("HTTP request/response cycle completed")
}

// startHealthMonitoring starts the health monitoring goroutine
func (t *Tunnel) startHealthMonitoring() {
	t.logger.Info("Starting health monitoring with %v interval", t.retryConfig.HealthCheckInterval)

	t.healthTicker = time.NewTicker(t.retryConfig.HealthCheckInterval)
	go func() {
		defer t.healthTicker.Stop()
		for {
			select {
			case <-t.healthTicker.C:
				// Simple health check - try to write to connection
				if t.conn != nil {
					t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					_, err := t.conn.Write([]byte{}) // Empty write as keepalive
					t.conn.SetWriteDeadline(time.Time{})

					if err != nil {
						t.logger.Info("Health check failed, triggering reconnection: %v", err)
						t.coordinatedReconnect()
						return
					}
				}

				// Update last ping time
				t.lastPing = time.Now()

			case <-t.ctx.Done():
				return
			}
		}
	}()
}

// coordinatedReconnect handles reconnection logic with coordination between HTTP and WebSocket handlers
func (t *Tunnel) coordinatedReconnect() {
	t.coordinatedReconnectWithContext(false)
}

// coordinatedReconnectWithContext handles reconnection with context about whether it's intentional
func (t *Tunnel) coordinatedReconnectWithContext(isIntentional bool) {
	// Use mutex to prevent multiple reconnection attempts
	t.reconnectMutex.Lock()
	defer t.reconnectMutex.Unlock()

	// If already reconnecting, don't start another attempt
	if t.isReconnecting {
		t.logger.Info("Reconnection already in progress, skipping")
		return
	}

	// If this is an automatic reconnection but we're doing an intentional reconnection, skip
	if !isIntentional && t.isIntentionalReconnect {
		t.logger.Info("Intentional reconnection in progress, skipping automatic reconnection")
		return
	}

	t.isReconnecting = true
	t.isIntentionalReconnect = isIntentional
	defer func() {
		t.isReconnecting = false
		t.isIntentionalReconnect = false
	}()

	if isIntentional {
		t.logger.Info("Starting intentional reconnection (WebSocket recycling) — preserving gRPC tunnel...")
	} else {
		t.logger.Info("Starting automatic reconnection (connection lost)...")
	}

	// Close both connections
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
	if t.wsConn != nil {
		t.wsConn.Close()
		t.wsConn = nil
	}

	// Close HTTP connections for large file streaming during reconnect
	if len(t.httpConnections) > 0 {
		for _, httpConn := range t.httpConnections {
			if httpConn != nil {
				httpConn.Close()
			}
		}
		t.httpConnections = nil
	}

	// Preserve gRPC client across reconnects to maintain stable client ID
	if t.grpcClient != nil {
		if isIntentional {
			t.logger.Info("[CLEANUP] Preserving gRPC client during WebSocket recycling")
		} else {
			t.logger.Info("[CLEANUP] Preserving existing gRPC client across reconnection (Client ID: %s)", t.grpcClient.GetClientID())
		}
	}

	// Stop health monitoring
	if t.healthTicker != nil {
		t.healthTicker.Stop()
		t.healthTicker = nil
	}

	// Start reconnection process
	go func() {
		// Load configuration for server address
		cfg, err := LoadConfig()
		if err != nil {
			cfg = &DefaultConfig // fallback to default
		}
		serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		t.connectWithRetry(serverAddr, nil)
	}()
}

// reconnect handles reconnection logic
func (t *Tunnel) reconnect() {
	// Delegate to coordinated reconnect for consistency
	t.coordinatedReconnect()
}

// Disconnect closes the tunnel connection and cleans up resources
func (t *Tunnel) Disconnect() error {
	// Release singleton lock
	if t.singletonManager != nil {
		if err := t.singletonManager.ReleaseLock(); err != nil {
			t.logger.Warn("Failed to release singleton lock: %v", err)
		}
	}

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

	// Close gRPC client if enabled
	var err error
	if t.grpcEnabled && t.grpcClient != nil {
		t.logger.Info("Closing gRPC tunnel client (Client ID: %s)...", t.grpcClient.GetClientID())
		if grpcErr := t.grpcClient.Stop(); grpcErr != nil {
			t.logger.Error("Error closing gRPC client: %v", grpcErr)
			err = grpcErr
		} else {
			t.logger.Info("✅ gRPC tunnel client closed successfully (Client ID: %s)", t.grpcClient.GetClientID())
		}
		t.grpcClient = nil
		t.grpcEnabled = false
	}

	// Close both TCP connections if they exist
	if t.conn != nil {
		// Set a deadline for graceful shutdown
		t.conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Don't send JSON control messages over HTTP connection
		// as they interfere with HTTP traffic

		// Close the HTTP connection
		tcpErr := t.conn.Close()
		if err == nil {
			err = tcpErr
		}
		t.conn = nil
	}

	if t.wsConn != nil {
		// Set a deadline for graceful shutdown
		t.wsConn.SetDeadline(time.Now().Add(5 * time.Second))

		// Close the WebSocket connection
		wsErr := t.wsConn.Close()
		t.wsConn = nil

		// Return the first error if any
		if err == nil {
			err = wsErr
		}
	}

	// Close HTTP connections for large file streaming
	if len(t.httpConnections) > 0 {
		t.logger.Info("Closing %d HTTP connections for large file streaming...", len(t.httpConnections))
		for i, httpConn := range t.httpConnections {
			if httpConn != nil {
				httpConn.SetDeadline(time.Now().Add(5 * time.Second))
				if closeErr := httpConn.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				t.logger.Debug("Closed HTTP connection %d for large files", i+1)
			}
		}
		t.httpConnections = nil
		t.logger.Info("✅ All HTTP connections for large files closed")
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
	t.logger.Info("Tunnel disconnected (hybrid mode: gRPC + TCP)")
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
		"state":              t.state.String(),
		"retry_count":        t.retryCount,
		"domain":             t.domain,
		"local_port":         t.localPort,
		"last_ping":          t.lastPing,
		"media_optimization": t.streamConfig.EnableMediaOptimization,
		"media_buffer_size":  t.streamConfig.MediaBufferSize,
		"grpc_enabled":       t.grpcEnabled,
		"tunnel_mode":        "hybrid", // New production-grade hybrid mode
	}

	if t.lastError != nil {
		stats["last_error"] = t.lastError.Error()
	}

	// Add gRPC client metrics if available
	if t.grpcEnabled && t.grpcClient != nil {
		grpcMetrics := t.grpcClient.GetMetrics()
		stats["grpc_metrics"] = grpcMetrics
		stats["grpc_connected"] = grpcMetrics["connected"]
		stats["grpc_requests"] = grpcMetrics["total_requests"]
		stats["grpc_responses"] = grpcMetrics["total_responses"]
		stats["grpc_errors"] = grpcMetrics["total_errors"]
		stats["grpc_timeout_errors"] = grpcMetrics["timeout_errors"]
		stats["grpc_reconnects"] = grpcMetrics["reconnect_count"]
		stats["grpc_timeout_reconnects"] = grpcMetrics["timeout_reconnects"]
	}

	return stats
}

// UpdateStreamingConfig updates the streaming configuration
func (t *Tunnel) UpdateStreamingConfig(config *StreamingConfig) {
	t.streamConfig = config
	t.logger.Info("Updated tunnel streaming configuration: MediaOptimization=%v, MediaBufferSize=%d",
		config.EnableMediaOptimization, config.MediaBufferSize)
}

// GetStreamingConfig returns the current streaming configuration
func (t *Tunnel) GetStreamingConfig() *StreamingConfig {
	return t.streamConfig
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

// isExpectedConnectionClose checks if the connection closure was expected
func isExpectedConnectionClose(err error) bool {
	if err == nil {
		return true
	}

	errStr := err.Error()
	expectedErrors := []string{
		"use of closed network connection",
		"connection reset by peer",
		"EOF",
		"broken pipe",
	}

	for _, expected := range expectedErrors {
		if strings.Contains(errStr, expected) {
			return true
		}
	}

	return false
}

// ConnectionStateProvider interface implementation

// GetConnectionCount returns the number of active connections
func (t *Tunnel) GetConnectionCount() int {
	count := 0

	// Count gRPC connection
	if t.grpcEnabled && t.grpcClient != nil {
		count++
	}

	// Count WebSocket connection
	if t.wsConn != nil {
		count++
	}

	// Count HTTP connections
	count += len(t.httpConnections)

	return count
}

// PreserveState captures the current tunnel state for restoration after update
func (t *Tunnel) PreserveState() (interface{}, error) {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()

	if t.state != StateConnected {
		return nil, fmt.Errorf("tunnel is not connected, cannot preserve state")
	}

	state := &TunnelState{
		Token:        t.token,
		Domain:       t.domain,
		LocalPort:    t.localPort,
		State:        t.state,
		Timestamp:    time.Now(),
		GRPCEnabled:  t.grpcEnabled,
		RetryConfig:  t.retryConfig,
		StreamConfig: t.streamConfig,
	}

	t.logger.Info("Preserved tunnel state for domain: %s, local port: %d", state.Domain, state.LocalPort)
	return state, nil
}

// RestoreState attempts to restore tunnel connection from preserved state
func (t *Tunnel) RestoreState(stateInterface interface{}) error {
	state, ok := stateInterface.(*TunnelState)
	if !ok {
		return fmt.Errorf("invalid state type for restoration")
	}

	t.logger.Info("Attempting to restore tunnel state for domain: %s", state.Domain)

	// Check if state is too old (more than 5 minutes)
	if time.Since(state.Timestamp) > 5*time.Minute {
		return fmt.Errorf("preserved state is too old (%v), cannot restore", time.Since(state.Timestamp))
	}

	// Stop current connections if any
	if t.IsConnected() {
		t.logger.Info("Stopping existing connections before restoration")
		t.Disconnect()
	}

	// Restore tunnel configuration
	t.token = state.Token
	t.domain = state.Domain
	t.localPort = state.LocalPort
	t.grpcEnabled = state.GRPCEnabled
	t.retryConfig = state.RetryConfig
	t.streamConfig = state.StreamConfig

	// Load current config to get server details and TLS config
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config for restoration: %w", err)
	}

	// Create new context for restored connection
	t.ctx, t.cancel = context.WithCancel(context.Background())

	// Reconstruct TLS config (since it's not serializable)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.Security.InsecureSkipVerify,
	}

	// Load certificates if available
	if cfg.Security.CACert != "" {
		if caCert, err := os.ReadFile(cfg.Security.CACert); err == nil {
			caCertPool := x509.NewCertPool()
			if caCertPool.AppendCertsFromPEM(caCert) {
				tlsConfig.RootCAs = caCertPool
			}
		}
	}

	if cfg.Security.ClientCert != "" && cfg.Security.ClientKey != "" {
		if cert, err := tls.LoadX509KeyPair(cfg.Security.ClientCert, cfg.Security.ClientKey); err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	// Attempt to reconnect
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	t.logger.Info("Reconnecting tunnel with preserved state...")
	err = t.Connect(t.ctx, serverAddr, t.token, t.domain, t.localPort, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to restore tunnel connection: %w", err)
	}

	t.logger.Info("Successfully restored tunnel connection")
	return nil
}

// SaveStateToFile saves tunnel state to a file for persistence across restarts
func (t *Tunnel) SaveStateToFile() error {
	state, err := t.PreserveState()
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".giraffecloud")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, "tunnel_state.json")
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	t.logger.Info("Saved tunnel state to file: %s", stateFile)
	return nil
}

// LoadStateFromFile loads tunnel state from a file
func (t *Tunnel) LoadStateFromFile() (interface{}, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	stateFile := filepath.Join(homeDir, ".giraffecloud", "tunnel_state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no saved state found")
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state TunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	t.logger.Info("Loaded tunnel state from file: %s", stateFile)
	return &state, nil
}

// ClearStateFile removes the saved state file
func (t *Tunnel) ClearStateFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	stateFile := filepath.Join(homeDir, ".giraffecloud", "tunnel_state.json")
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}

	t.logger.Debug("Cleared tunnel state file")
	return nil
}

// handleTunnelEstablishRequest handles tunnel establishment requests from the server
func (t *Tunnel) handleTunnelEstablishRequest(establishReq *proto.TunnelEstablishRequest) error {
	t.logger.Info("🔔 Received tunnel establishment request: %s for domain %s",
		establishReq.TunnelType.String(), establishReq.Domain)

	switch establishReq.TunnelType {
	case proto.TunnelType_TUNNEL_TYPE_TCP:
		return t.establishTCPTunnelOnDemand(establishReq)
	case proto.TunnelType_TUNNEL_TYPE_GRPC:
		return fmt.Errorf("gRPC tunnel already established")
	case proto.TunnelType_TUNNEL_TYPE_HYBRID:
		// For hybrid, we need both - but gRPC should already be established
		return t.establishTCPTunnelOnDemand(establishReq)
	default:
		return fmt.Errorf("unknown tunnel type: %s", establishReq.TunnelType.String())
	}
}

// establishTCPTunnelOnDemand establishes a TCP tunnel when requested by the server
func (t *Tunnel) establishTCPTunnelOnDemand(establishReq *proto.TunnelEstablishRequest) error {
	t.logger.Info("⚡ Establishing TCP tunnel on-demand for domain: %s", establishReq.Domain)

	// Check if TCP tunnel already exists
	if t.wsConn != nil {
		t.logger.Info("TCP tunnel already exists for domain: %s", establishReq.Domain)
		return nil
	}

	// Create TLS config for TCP connection
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	tlsConfig, err := CreateSecureTLSConfig(cfg.Security.CACert, cfg.Security.ClientCert, cfg.Security.ClientKey)
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	// Determine server address for TCP tunnel (port 4443)
	serverAddr := strings.Replace(t.grpcClient.serverAddr, ":4444", ":4443", 1)

	// Establish WebSocket tunnel connection
	wsConn, err := t.establishConnection(serverAddr, tlsConfig, "websocket")
	if err != nil {
		return fmt.Errorf("failed to establish TCP tunnel: %w", err)
	}

	t.wsConn = wsConn
	t.logger.Info("✅ TCP tunnel established successfully on-demand")

	// Start WebSocket handler
	t.wg.Add(1)
	go t.handleWebSocketConnection(wsConn)

	// Notify the hybrid router that TCP tunnel is now available
	t.notifyTCPTunnelEstablished()

	return nil
}

// notifyTCPTunnelEstablished notifies that TCP tunnel is ready (this would be wired to hybrid router)
func (t *Tunnel) notifyTCPTunnelEstablished() {
	// This will be wired to the hybrid router's OnTCPTunnelEstablished method
	// For now, just log that it's ready
	t.logger.Info("🚀 TCP tunnel ready - WebSocket connections can now be handled")
}
