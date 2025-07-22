package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// GRPCTunnelClient handles the client-side gRPC tunnel connection
// This replaces the connection pooling with a single high-performance stream
type GRPCTunnelClient struct {
	// Connection details
	serverAddr    string
	domain        string
	targetPort    int32
	token         string

	// gRPC connection
	conn   *grpc.ClientConn
	client proto.TunnelServiceClient
	stream proto.TunnelService_EstablishTunnelClient

	// State management
	connected     bool
	connecting    bool
	mu            sync.RWMutex
	stopChan      chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc

	// Request handling
	requestHandler    func(*proto.TunnelMessage) error
	responseChannels  map[string]chan *proto.TunnelMessage
	responseChannelsMu sync.RWMutex

	// Metrics
	totalRequests   int64
	totalResponses  int64
	totalErrors     int64
	reconnectCount  int64

	// Configuration
	config *GRPCClientConfig
	logger *logging.Logger
}

// GRPCClientConfig holds configuration for the gRPC client
type GRPCClientConfig struct {
	// Connection settings
	ConnectTimeout    time.Duration
	RequestTimeout    time.Duration
	KeepAliveTime     time.Duration
	KeepAliveTimeout  time.Duration

	// Retry settings
	MaxReconnectAttempts int
	ReconnectDelay       time.Duration
	BackoffMultiplier    float64

	// Security settings
	InsecureSkipVerify bool

	// Performance settings
	MaxMessageSize     int
	EnableCompression  bool
}

// DefaultGRPCClientConfig returns default client configuration
func DefaultGRPCClientConfig() *GRPCClientConfig {
	return &GRPCClientConfig{
		ConnectTimeout:       30 * time.Second,
		RequestTimeout:       30 * time.Second,
		KeepAliveTime:        30 * time.Second,
		KeepAliveTimeout:     10 * time.Second,
		MaxReconnectAttempts: -1, // Infinite retries
		ReconnectDelay:       1 * time.Second,
		BackoffMultiplier:    1.5,
		InsecureSkipVerify:   true, // For development
		MaxMessageSize:       16 * 1024 * 1024, // 16MB
		EnableCompression:    true,
	}
}

// NewGRPCTunnelClient creates a new gRPC tunnel client
func NewGRPCTunnelClient(serverAddr, domain, token string, targetPort int32, config *GRPCClientConfig) *GRPCTunnelClient {
	if config == nil {
		config = DefaultGRPCClientConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &GRPCTunnelClient{
		serverAddr:       serverAddr,
		domain:           domain,
		targetPort:       targetPort,
		token:            token,
		stopChan:         make(chan struct{}),
		ctx:              ctx,
		cancel:           cancel,
		responseChannels: make(map[string]chan *proto.TunnelMessage),
		config:           config,
		logger:           logging.GetGlobalLogger(),
	}

	return client
}

// SetRequestHandler sets the function to handle incoming HTTP requests
func (c *GRPCTunnelClient) SetRequestHandler(handler func(*proto.TunnelMessage) error) {
	c.requestHandler = handler
}

// Start establishes the gRPC tunnel connection
func (c *GRPCTunnelClient) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connecting || c.connected {
		return fmt.Errorf("client already started")
	}

	c.connecting = true
	defer func() { c.connecting = false }()

	if err := c.connect(); err != nil {
		return fmt.Errorf("failed to establish tunnel: %w", err)
	}

	c.connected = true
	c.logger.Info("gRPC tunnel established for domain: %s", c.domain)

	// Start message handling
	go c.handleIncomingMessages()
	go c.monitorConnection()

	return nil
}

// Stop gracefully stops the gRPC tunnel connection
func (c *GRPCTunnelClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.logger.Info("Stopping gRPC tunnel for domain: %s", c.domain)

	// Signal stop
	close(c.stopChan)
	c.cancel()

	// Close stream
	if c.stream != nil {
		c.stream.CloseSend()
	}

	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	c.logger.Info("gRPC tunnel stopped for domain: %s", c.domain)

	return nil
}

// IsConnected returns whether the tunnel is connected
func (c *GRPCTunnelClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// connect establishes the gRPC connection and stream
func (c *GRPCTunnelClient) connect() error {
	// Create TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
	}

	// Create gRPC connection
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.config.KeepAliveTime,
			Timeout:             c.config.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.config.MaxMessageSize),
			grpc.MaxCallSendMsgSize(c.config.MaxMessageSize),
		),
	}

	if c.config.EnableCompression {
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	}

	connectCtx, connectCancel := context.WithTimeout(c.ctx, c.config.ConnectTimeout)
	defer connectCancel()

	conn, err := grpc.DialContext(connectCtx, c.serverAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	c.client = proto.NewTunnelServiceClient(conn)

	// Establish tunnel stream
	stream, err := c.client.EstablishTunnel(c.ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to establish tunnel stream: %w", err)
	}

	c.stream = stream

	// Send handshake
	if err := c.sendHandshake(); err != nil {
		stream.CloseSend()
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	// Wait for handshake response
	if err := c.waitForHandshakeResponse(); err != nil {
		stream.CloseSend()
		conn.Close()
		return fmt.Errorf("handshake response failed: %w", err)
	}

	return nil
}

// sendHandshake sends the initial handshake message
func (c *GRPCTunnelClient) sendHandshake() error {
	handshake := &proto.TunnelMessage{
		RequestId: generateRequestID(),
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_Control{
			Control: &proto.TunnelControl{
				ControlType: &proto.TunnelControl_Handshake{
					Handshake: &proto.TunnelHandshake{
						Token:          c.token,
						Domain:         c.domain,
						TargetPort:     c.targetPort,
						Capabilities:   []string{"http", "compression"},
						ClientVersion:  "1.0.0",
					},
				},
			},
		},
	}

	return c.stream.Send(handshake)
}

// waitForHandshakeResponse waits for and validates the handshake response
func (c *GRPCTunnelClient) waitForHandshakeResponse() error {
	// Set timeout for handshake response
	responseChan := make(chan *proto.TunnelMessage, 1)
	errChan := make(chan error, 1)

	go func() {
		msg, err := c.stream.Recv()
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- msg
	}()

	select {
	case msg := <-responseChan:
		// Validate handshake response
		if control := msg.GetControl(); control != nil {
			if status := control.GetStatus(); status != nil {
				if status.State == proto.TunnelState_TUNNEL_STATE_CONNECTED {
					c.logger.Info("Handshake successful for domain: %s", c.domain)
					return nil
				}
				return fmt.Errorf("handshake failed: %s", status.ErrorMessage)
			}
		}
		return fmt.Errorf("invalid handshake response")

	case err := <-errChan:
		return fmt.Errorf("failed to receive handshake response: %w", err)

	case <-time.After(10 * time.Second):
		return fmt.Errorf("handshake timeout")

	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// handleIncomingMessages handles messages from the server
func (c *GRPCTunnelClient) handleIncomingMessages() {
	for {
		select {
		case <-c.stopChan:
			return
		case <-c.ctx.Done():
			return
		default:
		}

		msg, err := c.stream.Recv()
		if err != nil {
			if err == io.EOF {
				c.logger.Info("Server closed the tunnel stream")
			} else if status.Code(err) == codes.Canceled {
				c.logger.Info("Tunnel stream canceled")
			} else {
				c.logger.Error("Error receiving message: %v", err)
				atomic.AddInt64(&c.totalErrors, 1)
			}

			// Trigger reconnection if not stopping
			select {
			case <-c.stopChan:
				return
			default:
				go c.reconnect()
				return
			}
		}

		// Handle the message
		if err := c.handleMessage(msg); err != nil {
			c.logger.Error("Error handling message: %v", err)
			atomic.AddInt64(&c.totalErrors, 1)
		}
	}
}

// handleMessage handles a single message from the server
func (c *GRPCTunnelClient) handleMessage(msg *proto.TunnelMessage) error {
	switch msgType := msg.MessageType.(type) {
	case *proto.TunnelMessage_HttpRequest:
		// Handle HTTP request from server
		return c.handleHTTPRequest(msg)

	case *proto.TunnelMessage_Control:
		// Handle control message
		return c.handleControlMessage(msg)

	case *proto.TunnelMessage_Error:
		// Handle error message
		c.logger.Error("Received error from server: %s (code: %d)",
			msgType.Error.Message, msgType.Error.Code)
		return nil

	default:
		c.logger.Warn("Unknown message type: %T", msgType)
		return nil
	}
}

// handleHTTPRequest handles an HTTP request from the server
func (c *GRPCTunnelClient) handleHTTPRequest(msg *proto.TunnelMessage) error {
	atomic.AddInt64(&c.totalRequests, 1)

	if c.requestHandler != nil {
		return c.requestHandler(msg)
	}

	// Default handler: forward to local service
	return c.forwardToLocalService(msg)
}

// forwardToLocalService forwards the request to the local service
func (c *GRPCTunnelClient) forwardToLocalService(msg *proto.TunnelMessage) error {
	httpReq := msg.GetHttpRequest()
	if httpReq == nil {
		return fmt.Errorf("invalid HTTP request message")
	}

	// Build URL for local service
	url := fmt.Sprintf("http://127.0.0.1:%d%s", c.targetPort, httpReq.Path)

	// Create HTTP request
	req, err := http.NewRequest(httpReq.Method, url, strings.NewReader(string(httpReq.Body)))
	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Set headers
	for key, value := range httpReq.Headers {
		req.Header.Set(key, value)
	}

	// Make request to local service
	client := &http.Client{
		Timeout: c.config.RequestTimeout,
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	processingTime := time.Since(startTime)

	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Request failed: %v", err))
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Failed to read response: %v", err))
	}

	// Convert headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Send response back to server
	response := &proto.TunnelMessage{
		RequestId: msg.RequestId,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpResponse{
			HttpResponse: &proto.HTTPResponse{
				StatusCode: int32(resp.StatusCode),
				StatusText: resp.Status,
				Headers:    headers,
				Body:       body,
				Metadata: &proto.ResponseMetadata{
					ProcessingTimeMs: processingTime.Milliseconds(),
					ResponseSize:     int64(len(body)),
					CacheStatus:      proto.CacheStatus_CACHE_STATUS_MISS,
				},
			},
		},
	}

	atomic.AddInt64(&c.totalResponses, 1)
	return c.stream.Send(response)
}

// sendErrorResponse sends an error response back to the server
func (c *GRPCTunnelClient) sendErrorResponse(requestId, errorMsg string) error {
	response := &proto.TunnelMessage{
		RequestId: requestId,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_Error{
			Error: &proto.ErrorMessage{
				Code:      500,
				Message:   errorMsg,
				Retryable: false,
			},
		},
	}

	return c.stream.Send(response)
}

// handleControlMessage handles control messages from the server
func (c *GRPCTunnelClient) handleControlMessage(msg *proto.TunnelMessage) error {
	control := msg.GetControl()
	if control == nil {
		return nil
	}

	switch controlType := control.ControlType.(type) {
	case *proto.TunnelControl_Status:
		// Health check response
		c.logger.Debug("Received status update: %s", controlType.Status.State)

	case *proto.TunnelControl_Config:
		// Configuration update
		config := controlType.Config
		c.logger.Info("Received config update: max_concurrent=%d, timeout=%ds",
			config.MaxConcurrent, config.TimeoutSeconds)

	default:
		c.logger.Debug("Unknown control message type: %T", controlType)
	}

	return nil
}

// monitorConnection monitors the connection health
func (c *GRPCTunnelClient) monitorConnection() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Report metrics
			c.reportMetrics()
		}
	}
}

// reportMetrics reports client metrics
func (c *GRPCTunnelClient) reportMetrics() {
	total := atomic.LoadInt64(&c.totalRequests)
	responses := atomic.LoadInt64(&c.totalResponses)
	errors := atomic.LoadInt64(&c.totalErrors)
	reconnects := atomic.LoadInt64(&c.reconnectCount)

	c.logger.Info("[gRPC CLIENT] Domain: %s, Requests: %d, Responses: %d, Errors: %d, Reconnects: %d",
		c.domain, total, responses, errors, reconnects)
}

// reconnect attempts to reconnect the tunnel
func (c *GRPCTunnelClient) reconnect() {
	atomic.AddInt64(&c.reconnectCount, 1)

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return // Already disconnected
	}

	c.logger.Warn("Attempting to reconnect gRPC tunnel for domain: %s", c.domain)

	// Close existing connections
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false

	// Retry connection with exponential backoff
	delay := c.config.ReconnectDelay
	attempts := 0

	for {
		select {
		case <-c.stopChan:
			return
		case <-c.ctx.Done():
			return
		default:
		}

		attempts++
		if c.config.MaxReconnectAttempts > 0 && attempts > c.config.MaxReconnectAttempts {
			c.logger.Error("Max reconnection attempts reached for domain: %s", c.domain)
			return
		}

		c.logger.Info("Reconnection attempt #%d for domain: %s", attempts, c.domain)

		if err := c.connect(); err != nil {
			c.logger.Error("Reconnection failed: %v", err)

			// Exponential backoff
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * c.config.BackoffMultiplier)
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			continue
		}

		c.connected = true
		c.logger.Info("Successfully reconnected gRPC tunnel for domain: %s", c.domain)

		// Restart message handling
		go c.handleIncomingMessages()
		return
	}
}

// GetMetrics returns current client metrics
func (c *GRPCTunnelClient) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"connected":        c.connected,
		"total_requests":   atomic.LoadInt64(&c.totalRequests),
		"total_responses":  atomic.LoadInt64(&c.totalResponses),
		"total_errors":     atomic.LoadInt64(&c.totalErrors),
		"reconnect_count":  atomic.LoadInt64(&c.reconnectCount),
		"domain":           c.domain,
		"target_port":      c.targetPort,
	}
}