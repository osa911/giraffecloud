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
		InsecureSkipVerify:   false, // PRODUCTION: Use proper certificate validation
		MaxMessageSize:       16 * 1024 * 1024, // 16MB - small files only, large files use chunked streaming
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
	// Load configuration for certificate paths and create secure TLS configuration
	var tlsConfig *tls.Config
	cfg, err := LoadConfig()
	if err != nil {
		c.logger.Warn("Failed to load config for certificates, using insecure connection: %v", err)
		// Fallback to insecure configuration
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	} else {
		// Create secure TLS configuration with proper certificates
		tlsConfig, err = CreateSecureTLSConfig(cfg.Security.CACert, cfg.Security.ClientCert, cfg.Security.ClientKey)
		if err != nil {
			c.logger.Warn("Failed to create secure TLS config, using insecure fallback: %v", err)
			// Fallback to insecure configuration
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
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
						Capabilities: &proto.TunnelCapabilities{
							SupportsChunkedStreaming: true,
							SupportsCompression:      true,
							MaxChunkSize:            1024 * 1024, // 1MB chunks
							SupportedEncodings:      []string{"gzip", "deflate"},
						},
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

	// Check if this is a large file request that should use chunked streaming
	if httpReq.IsLargeFile && c.shouldUseChunkedStreaming(httpReq) {
		c.logger.Info("[CHUNKED CLIENT] 🚀 Processing large file with chunked streaming: %s %s",
			httpReq.Method, httpReq.Path)
		return c.forwardLargeFileWithChunking(msg, httpReq)
	}

	// Regular processing for small files
	return c.forwardRegularRequest(msg, httpReq)
}

// shouldUseChunkedStreaming determines if we should use chunked streaming
// PERFECT BINARY RULE: Files >16MB = Chunked Streaming (UNLIMITED), Files ≤16MB = Regular gRPC (16MB)
func (c *GRPCTunnelClient) shouldUseChunkedStreaming(httpReq *proto.HTTPRequest) bool {
	// Always use chunked streaming when explicitly marked as large file
	if httpReq.IsLargeFile {
		c.logger.Debug("[CHUNKED CLIENT] Explicitly marked as large file → UNLIMITED STREAMING")
		return true
	}

		// Check file extensions that are typically large (>16MB)
	path := strings.ToLower(httpReq.Path)
	typicallyLargeExtensions := []string{
		// Video files - almost always >16MB
		".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4v", ".flv", ".wmv", ".mpg", ".mpeg", ".m2v",
		// Archives - often >16MB
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz",
		// Large binaries and disk images - usually >16MB
		".iso", ".img", ".dmg", ".exe", ".msi", ".deb", ".rpm", ".appimage",
		// Large audio files - often >16MB
		".wav", ".flac", ".ape",
	}

	for _, ext := range typicallyLargeExtensions {
		if strings.HasSuffix(path, ext) {
			c.logger.Debug("[CHUNKED CLIENT] Large file extension: %s → UNLIMITED STREAMING", ext)
			return true
		}
	}

	// Check path patterns that typically serve large files
	largeFilePaths := []string{
		"/video/", "/videos/", "/movie/", "/movies/", "/playback",
		"/download/", "/downloads/", "/file/", "/files/",
		"/original/", "/raw/", "/backup/", "/archive/",
		"/media/large/", "/assets/large/", "/content/large/",
	}
	for _, largePath := range largeFilePaths {
		if strings.Contains(path, largePath) {
			c.logger.Debug("[CHUNKED CLIENT] Large file path: %s → UNLIMITED STREAMING", largePath)
			return true
		}
	}

	// Default: Use regular gRPC for small files
	c.logger.Debug("[REGULAR CLIENT] Small file → Regular gRPC (16MB limit)")
	return false
}

// forwardLargeFileWithChunking handles large file requests with streaming
func (c *GRPCTunnelClient) forwardLargeFileWithChunking(msg *proto.TunnelMessage, httpReq *proto.HTTPRequest) error {
	c.logger.Info("[CHUNKED CLIENT] 📦 Implementing chunked response streaming for unlimited file size")

	// Make request to local service
	response, err := c.makeLocalServiceRequest(httpReq)
	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Local service request failed: %v", err))
	}
	defer response.Body.Close()

	// Stream the response back in chunks for unlimited file size support
	return c.streamResponseInChunks(msg.RequestId, response)
}

// forwardRegularRequest handles regular (small file) requests
func (c *GRPCTunnelClient) forwardRegularRequest(msg *proto.TunnelMessage, httpReq *proto.HTTPRequest) error {
	response, err := c.makeLocalServiceRequest(httpReq)
	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Local service request failed: %v", err))
	}
	defer response.Body.Close()

	// Read entire response for regular files
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return c.sendErrorResponse(msg.RequestId, fmt.Sprintf("Failed to read response: %v", err))
	}

	// Send single response back to server
	return c.sendCompleteResponse(msg.RequestId, response, body)
}

// makeLocalServiceRequest makes the actual HTTP request to the local service
func (c *GRPCTunnelClient) makeLocalServiceRequest(httpReq *proto.HTTPRequest) (*http.Response, error) {
	// Build URL for local service
	url := fmt.Sprintf("http://127.0.0.1:%d%s", c.targetPort, httpReq.Path)

	// Create HTTP request
	req, err := http.NewRequest(httpReq.Method, url, strings.NewReader(string(httpReq.Body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range httpReq.Headers {
		req.Header.Set(key, value)
	}

	// Make request to local service with generous timeout
	client := &http.Client{
		Timeout: 120 * time.Second, // 2 minutes for large files
	}

	startTime := time.Now()
	c.logger.Debug("[gRPC CLIENT] Forwarding request to local service: %s %s", httpReq.Method, httpReq.Path)

	resp, err := client.Do(req)
	processingTime := time.Since(startTime)

	if err != nil {
		c.logger.Error("[gRPC CLIENT] Local service request failed after %v: %v", processingTime, err)
		return nil, err
	}

	c.logger.Debug("[gRPC CLIENT] Local service responded in %v: %d %s",
		processingTime, resp.StatusCode, httpReq.Path)

	return resp, nil
}

// streamResponseInChunks streams large responses in chunks for unlimited file size
func (c *GRPCTunnelClient) streamResponseInChunks(requestID string, response *http.Response) error {
	const ChunkSize = 1024 * 1024 // 1MB chunks

	c.logger.Info("[CHUNKED CLIENT] 📡 Streaming response in %dKB chunks (UNLIMITED SIZE)", ChunkSize/1024)

	// Convert headers
	headers := make(map[string]string)
	for key, values := range response.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	chunkNum := 0
	buffer := make([]byte, ChunkSize)

	for {
		// Read chunk from response
		n, err := response.Body.Read(buffer)
		if n > 0 {
			chunkNum++

			// Create chunk data
			chunkData := make([]byte, n)
			copy(chunkData, buffer[:n])

			// Determine chunk ID (mark final chunk appropriately)
			var chunkId string
			isFinalChunk := (err == io.EOF)
			if isFinalChunk {
				chunkId = fmt.Sprintf("chunk-%d_final", chunkNum)
			} else {
				chunkId = fmt.Sprintf("chunk-%d", chunkNum)
			}

			// Send chunk response
			chunkResponse := &proto.TunnelMessage{
				RequestId: requestID,
				Timestamp: time.Now().Unix(),
				MessageType: &proto.TunnelMessage_HttpResponse{
					HttpResponse: &proto.HTTPResponse{
						StatusCode: int32(response.StatusCode),
						StatusText: response.Status,
						Headers:    headers,
						Body:       chunkData,
						IsChunked:  true,
						ChunkId:    chunkId,
					},
				},
			}

			c.logger.Debug("[CHUNKED CLIENT] ✅ Sending chunk %d (%d bytes), final: %v", chunkNum, len(chunkData), isFinalChunk)

			if sendErr := c.stream.Send(chunkResponse); sendErr != nil {
				c.logger.Error("[CHUNKED CLIENT] Failed to send chunk %d: %v", chunkNum, sendErr)
				return sendErr
			}
		}

		// Check for end of file
		if err == io.EOF {
			c.logger.Info("[CHUNKED CLIENT] 🎉 Completed streaming %d chunks for large file", chunkNum)
			break
		} else if err != nil {
			c.logger.Error("[CHUNKED CLIENT] Error reading response: %v", err)
			return c.sendErrorResponse(requestID, fmt.Sprintf("Failed to read response: %v", err))
		}
	}

	atomic.AddInt64(&c.totalResponses, 1)
	return nil
}

// sendCompleteResponse sends a complete response for regular files
func (c *GRPCTunnelClient) sendCompleteResponse(requestID string, response *http.Response, body []byte) error {
	// Convert headers
	headers := make(map[string]string)
	for key, values := range response.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Send response back to server
	responseMsg := &proto.TunnelMessage{
		RequestId: requestID,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_HttpResponse{
			HttpResponse: &proto.HTTPResponse{
				StatusCode: int32(response.StatusCode),
				StatusText: response.Status,
				Headers:    headers,
				Body:       body,
				Metadata: &proto.ResponseMetadata{
					ProcessingTimeMs: 0, // We can add timing if needed
					ResponseSize:     int64(len(body)),
					CacheStatus:      proto.CacheStatus_CACHE_STATUS_MISS,
				},
			},
		},
	}

	atomic.AddInt64(&c.totalResponses, 1)
	return c.stream.Send(responseMsg)
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