package tunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/tunnel/proto"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Legacy ChunkedResponseBuffer removed - now using memory-efficient streaming via io.Pipe()

// GRPCTunnelServer implements the production-grade gRPC tunnel service
// Designed for high-performance, unlimited concurrency, and Cloudflare-level reliability
type GRPCTunnelServer struct {
	proto.UnimplementedTunnelServiceServer

	// Core dependencies
	logger        *logging.Logger
	tokenRepo     repository.TokenRepository
	tunnelRepo    repository.TunnelRepository
	tunnelService interfaces.TunnelService

	// gRPC server
	grpcServer *grpc.Server
	listener   net.Listener

	// Active tunnel streams (domain -> stream connection)
	tunnelStreams    map[string]*TunnelStream
	tunnelStreamsMux sync.RWMutex

	// Performance metrics (atomic for thread safety)
	totalRequests  int64
	concurrentReqs int64
	totalResponses int64
	totalErrors    int64
	timeoutErrors  int64
	totalBytesIn   int64
	totalBytesOut  int64

	// Configuration
	config *GRPCTunnelConfig

	// Security and rate limiting
	rateLimiter *RateLimiter
	security    *SecurityMiddleware

	// Usage aggregation
	usage UsageRecorder
	quota QuotaChecker
}

// SetUsageRecorder wires a usage recorder for accounting.
func (s *GRPCTunnelServer) SetUsageRecorder(rec UsageRecorder) {
	s.usage = rec
}

// SetQuotaChecker wires quota checker
func (s *GRPCTunnelServer) SetQuotaChecker(q QuotaChecker) { s.quota = q }

// TunnelStream represents an active tunnel connection
type TunnelStream struct {
	Domain     string
	TargetPort int32
	TunnelID   uint32
	Stream     proto.TunnelService_EstablishTunnelServer
	Context    context.Context
	UserID     uint32

	// Request correlation (requestID -> response channel)
	pendingRequests map[string]chan *proto.TunnelMessage
	requestsMux     sync.RWMutex

	// Note: Chunked response handling now uses memory-efficient streaming via io.Pipe()

	// Stream state
	connected     bool
	lastActivity  time.Time
	totalRequests int64
	totalErrors   int64

	// Concurrency control
	mu sync.RWMutex
}

// GRPCTunnelConfig holds configuration for the gRPC tunnel service
type GRPCTunnelConfig struct {
	// Server settings
	MaxConcurrentStreams uint32
	MaxMessageSize       int
	KeepAliveTimeout     time.Duration
	KeepAliveInterval    time.Duration

	// Request settings
	RequestTimeout  time.Duration
	MaxRequestSize  int64
	MaxResponseSize int64

	// Security settings
	RequireAuthentication bool
	AllowedOrigins        []string
	RateLimitRPM          int
	RateLimitBurst        int
}

// DefaultGRPCTunnelConfig returns production-ready default configuration
func DefaultGRPCTunnelConfig() *GRPCTunnelConfig {
	return &GRPCTunnelConfig{
		MaxConcurrentStreams:  5000,             // 5000 concurrent users - conservative and safe (~75 MB memory)
		MaxMessageSize:        16 * 1024 * 1024, // 16MB - small files only, large files use chunked streaming
		KeepAliveTimeout:      30 * time.Second,
		KeepAliveInterval:     5 * time.Second,
		RequestTimeout:        30 * time.Second,
		MaxRequestSize:        16 * 1024 * 1024, // 16MB - small files only
		MaxResponseSize:       16 * 1024 * 1024, // 16MB - small files only
		RequireAuthentication: true,
		RateLimitRPM:          5000, // 5000 requests per minute per tunnel
		RateLimitBurst:        500,  // 500 requests per minute per tunnel
	}
}

// NewGRPCTunnelServer creates a new production-grade gRPC tunnel server
func NewGRPCTunnelServer(
	tokenRepo repository.TokenRepository,
	tunnelRepo repository.TunnelRepository,
	tunnelService interfaces.TunnelService,
	config *GRPCTunnelConfig,
) *GRPCTunnelServer {
	if config == nil {
		config = DefaultGRPCTunnelConfig()
	}

	server := &GRPCTunnelServer{
		logger:        logging.GetGlobalLogger(),
		tokenRepo:     tokenRepo,
		tunnelRepo:    tunnelRepo,
		tunnelService: tunnelService,
		tunnelStreams: make(map[string]*TunnelStream),
		config:        config,
		rateLimiter:   NewRateLimiter(config.RateLimitRPM, config.RateLimitBurst),
		security:      NewSecurityMiddleware(),
	}

	return server
}

// Start starts the gRPC tunnel server
func (s *GRPCTunnelServer) Start(addr string) error {
	// Create secure TLS configuration with mutual authentication
	tlsConfig, err := CreateSecureServerTLSConfig("/app/certs/tunnel.crt", "/app/certs/tunnel.key", "/app/certs/ca.crt")
	if err != nil {
		s.logger.Warn("Failed to create secure TLS config, using fallback: %v", err)
		// Fallback configuration for compatibility
		tlsConfig = &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			},
		}
	} else {
		s.logger.Info("🔐 gRPC Server using PRODUCTION-GRADE TLS with mutual authentication")
	}

	creds := credentials.NewTLS(tlsConfig)

	// Create gRPC server with production settings
	s.grpcServer = grpc.NewServer(
		grpc.Creds(creds),
		grpc.MaxConcurrentStreams(s.config.MaxConcurrentStreams),
		grpc.MaxRecvMsgSize(s.config.MaxMessageSize),
		grpc.MaxSendMsgSize(s.config.MaxMessageSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    s.config.KeepAliveInterval,
			Timeout: s.config.KeepAliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	// Register the tunnel service
	proto.RegisterTunnelServiceServer(s.grpcServer, s)

	// Create listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Start server in background
	go func() {
		s.logger.Info("gRPC Tunnel Server starting on %s", addr)
		if err := s.grpcServer.Serve(s.listener); err != nil {
			s.logger.Error("gRPC server error: %v", err)
		}
	}()

	// Start metrics reporting
	go s.reportMetrics()

	s.logger.Info("gRPC Tunnel Server started successfully")
	return nil
}

// Stop gracefully stops the gRPC tunnel server
func (s *GRPCTunnelServer) Stop() error {
	s.logger.Info("Stopping gRPC Tunnel Server...")

	if s.grpcServer != nil {
		// Graceful stop with timeout
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()

		// Force stop after timeout
		select {
		case <-stopped:
			s.logger.Info("gRPC server stopped gracefully")
		case <-time.After(30 * time.Second):
			s.logger.Warn("Force stopping gRPC server after timeout")
			s.grpcServer.Stop()
		}
	}

	return nil
}

// EstablishTunnel implements the main gRPC streaming method for tunnel communication
func (s *GRPCTunnelServer) EstablishTunnel(stream proto.TunnelService_EstablishTunnelServer) error {
	ctx := stream.Context()
	s.logger.Info("New tunnel establishment request from %s", getPeerIP(ctx))

	// Wait for handshake message
	handshakeMsg, err := stream.Recv()
	if err != nil {
		s.logger.Error("Failed to receive handshake: %v", err)
		return status.Errorf(codes.InvalidArgument, "handshake required")
	}

	// Validate handshake
	handshake := handshakeMsg.GetControl().GetHandshake()
	if handshake == nil {
		return status.Errorf(codes.InvalidArgument, "invalid handshake message")
	}

	// Authenticate the tunnel
	tunnel, err := s.authenticateTunnel(ctx, handshake)
	if err != nil {
		s.logger.Error("Tunnel authentication failed: %v", err)
		return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	s.logger.Info("Authenticated tunnel for domain: %s, user: %d", tunnel.Domain, tunnel.UserID)

	// CRITICAL: Update client IP and trigger Caddy configuration (RESTORED FROM OLD HANDSHAKE)
	clientIP := getPeerIP(ctx)
	if s.tunnelService != nil {
		s.logger.Info("🔧 Updating client IP and configuring Caddy for domain: %s -> %s", tunnel.Domain, clientIP)
		if err := s.tunnelService.UpdateClientIP(ctx, uint32(tunnel.ID), clientIP); err != nil {
			s.logger.Error("Failed to update client IP and configure Caddy: %v", err)
			return status.Errorf(codes.Internal, "failed to configure tunnel: %v", err)
		}
		s.logger.Info("✅ Successfully configured Caddy route for domain: %s", tunnel.Domain)
	} else {
		s.logger.Warn("⚠️  Tunnel service not available - Caddy configuration skipped")
	}

	// Create tunnel stream
	// If client didn't send target port, use server-side configured target port
	chosenPort := handshake.TargetPort
	if chosenPort == 0 {
		chosenPort = int32(tunnel.TargetPort)
	}
	tunnelStream := &TunnelStream{
		Domain:          tunnel.Domain,
		TargetPort:      chosenPort,
		TunnelID:        uint32(tunnel.ID),
		Stream:          stream,
		Context:         ctx,
		UserID:          tunnel.UserID,
		pendingRequests: make(map[string]chan *proto.TunnelMessage),
		connected:       true,
		lastActivity:    time.Now(),
	}

	// Register tunnel stream
	s.tunnelStreamsMux.Lock()
	s.tunnelStreams[tunnel.Domain] = tunnelStream
	s.tunnelStreamsMux.Unlock()

	defer func() {
		s.tunnelStreamsMux.Lock()
		delete(s.tunnelStreams, tunnel.Domain)
		s.tunnelStreamsMux.Unlock()

		// Clean up all pending requests and chunked streaming state
		s.cleanupTunnelStreamState(tunnelStream)

		// CRITICAL: Remove Caddy route when tunnel disconnects (RESTORED FROM OLD HANDSHAKE)
		if s.tunnelService != nil {
			s.logger.Info("🔧 Removing Caddy route for disconnected tunnel: %s", tunnel.Domain)
			if err := s.tunnelService.UpdateClientIP(ctx, uint32(tunnel.ID), ""); err != nil {
				s.logger.Error("Failed to remove Caddy route: %v", err)
			} else {
				s.logger.Info("✅ Successfully removed Caddy route for domain: %s", tunnel.Domain)
			}
		}

		s.logger.Info("Tunnel disconnected for domain: %s (all state cleaned up)", tunnel.Domain)
	}()

	// Send handshake response (ENHANCED: Include success confirmation like old handshake)
	handshakeResponse := &proto.TunnelMessage{
		RequestId: handshakeMsg.RequestId,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_Control{
			Control: &proto.TunnelControl{
				ControlType: &proto.TunnelControl_Status{
					Status: &proto.TunnelStatus{
						State:             proto.TunnelState_TUNNEL_STATE_CONNECTED,
						Domain:            tunnel.Domain,
						TargetPort:        chosenPort,
						ConnectedAt:       time.Now().Unix(),
						ActiveConnections: 1,
						LastActivity:      time.Now().Unix(),
					},
				},
			},
		},
	}

	if err := stream.Send(handshakeResponse); err != nil {
		s.logger.Error("Failed to send handshake response: %v", err)
		return err
	}

	// Handle incoming messages from client
	go s.handleClientMessages(tunnelStream)

	// Keep the stream alive and monitor health
	return s.monitorTunnelHealth(tunnelStream)
}

// HealthCheck implements health checking for the tunnel service
func (s *GRPCTunnelServer) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	// Check system health
	activeStreams := len(s.tunnelStreams)
	concurrent := atomic.LoadInt64(&s.concurrentReqs)

	status := proto.HealthStatus_HEALTH_STATUS_SERVING
	message := fmt.Sprintf("Healthy - %d active tunnels, %d concurrent requests", activeStreams, concurrent)

	// Check if system is overloaded
	if concurrent > 10000 { // Configurable threshold
		status = proto.HealthStatus_HEALTH_STATUS_NOT_SERVING
		message = "System overloaded"
	}

	return &proto.HealthCheckResponse{
		Status:  status,
		Message: message,
		Details: map[string]string{
			"active_tunnels":      fmt.Sprintf("%d", activeStreams),
			"concurrent_requests": fmt.Sprintf("%d", concurrent),
			"total_requests":      fmt.Sprintf("%d", atomic.LoadInt64(&s.totalRequests)),
			"total_errors":        fmt.Sprintf("%d", atomic.LoadInt64(&s.totalErrors)),
			"timeout_errors":      fmt.Sprintf("%d", atomic.LoadInt64(&s.timeoutErrors)),
		},
	}, nil
}

// ProxyHTTPRequest handles HTTP request proxying through the gRPC tunnel
func (s *GRPCTunnelServer) ProxyHTTPRequest(domain string, req *http.Request, clientIP string) (*http.Response, error) {
	atomic.AddInt64(&s.totalRequests, 1)
	atomic.AddInt64(&s.concurrentReqs, 1)
	defer atomic.AddInt64(&s.concurrentReqs, -1)

	// Get tunnel stream for domain
	s.tunnelStreamsMux.RLock()
	tunnelStream, exists := s.tunnelStreams[domain]
	s.tunnelStreamsMux.RUnlock()

	if !exists || !tunnelStream.connected {
		atomic.AddInt64(&s.totalErrors, 1)
		return nil, fmt.Errorf("no active tunnel for domain: %s", domain)
	}

	// Quota/Rate limiting check
	if s.quota != nil {
		// Lookup stream to get user ID
		s.tunnelStreamsMux.RLock()
		ts, ok := s.tunnelStreams[domain]
		s.tunnelStreamsMux.RUnlock()
		if ok {
			res, _ := s.quota.CheckUser(context.Background(), ts.UserID)
			switch res.Decision {
			case QuotaBlock:
				return &http.Response{
					StatusCode: http.StatusPaymentRequired,
					Status:     fmt.Sprintf("%d %s", http.StatusPaymentRequired, http.StatusText(http.StatusPaymentRequired)),
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader([]byte("Quota exceeded"))),
				}, nil
			case QuotaWarn:
				// Add header to warn client
				req.Header.Set("X-Quota-Warn", "true")
			}
		}
	}
	// Rate limiter check
	if !s.rateLimiter.Allow(domain) {
		atomic.AddInt64(&s.totalErrors, 1)
		return nil, fmt.Errorf("rate limit exceeded for domain: %s", domain)
	}

	// Convert HTTP request to protobuf message
	grpcReq, err := s.httpToGRPC(req, clientIP)
	if err != nil {
		atomic.AddInt64(&s.totalErrors, 1)
		return nil, fmt.Errorf("failed to convert HTTP request: %w", err)
	}

	// Send request and wait for response
	response, err := s.sendRequestAndWaitResponse(tunnelStream, grpcReq)
	if err != nil {
		atomic.AddInt64(&s.totalErrors, 1)
		if isTimeoutError(err) {
			atomic.AddInt64(&s.timeoutErrors, 1)
		}
		return nil, err
	}

	// Usage: count request and rough request bytes
	if s.usage != nil {
		var reqBytes int64
		if b := grpcReq.GetHttpRequest().Body; len(b) > 0 {
			reqBytes = int64(len(b))
		}
		s.usage.Increment(tunnelStream.UserID, tunnelStream.TunnelID, domain, reqBytes, 0, 1)
	}
	atomic.AddInt64(&s.totalResponses, 1)
	return response, nil
}

// cleanupTunnelStreamState cleans up all state associated with a tunnel stream
// This prevents stale state from interfering with reconnections
func (s *GRPCTunnelServer) cleanupTunnelStreamState(tunnelStream *TunnelStream) {
	s.logger.Info("[CLEANUP] 🧹 Cleaning up tunnel stream state for domain: %s", tunnelStream.Domain)

	// Clean up all pending requests to prevent goroutine leaks
	tunnelStream.requestsMux.Lock()
	pendingCount := len(tunnelStream.pendingRequests)

	// Close all response channels and clear pending requests
	for requestID, responseChan := range tunnelStream.pendingRequests {
		s.logger.Debug("[CLEANUP] 🚮 Cleaning up pending request: %s", requestID)

		// Safely close the channel to signal any waiting goroutines
		func(ch chan *proto.TunnelMessage, id string) {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Debug("[CLEANUP] Channel already closed for request: %s", id)
				}
			}()
			close(ch)
		}(responseChan, requestID)

		// Remove from pending requests
		delete(tunnelStream.pendingRequests, requestID)
	}

	tunnelStream.requestsMux.Unlock()

	if pendingCount > 0 {
		s.logger.Info("[CLEANUP] ✅ Cleaned up %d pending requests for domain: %s", pendingCount, tunnelStream.Domain)
		s.logger.Info("[CLEANUP] 🔄 Ready for clean reconnection - no stale state")
	} else {
		s.logger.Debug("[CLEANUP] ✅ No pending requests to clean up for domain: %s", tunnelStream.Domain)
	}
}

// SendTunnelEstablishRequest sends a tunnel establishment request to the client
func (s *GRPCTunnelServer) SendTunnelEstablishRequest(domain string, establishReq *proto.TunnelEstablishRequest) error {
	// Get tunnel stream for domain
	s.tunnelStreamsMux.RLock()
	tunnelStream, exists := s.tunnelStreams[domain]
	s.tunnelStreamsMux.RUnlock()

	if !exists || !tunnelStream.connected {
		return fmt.Errorf("no active gRPC tunnel for domain: %s", domain)
	}

	// Create control message with establishment request
	controlMsg := &proto.TunnelMessage{
		RequestId: establishReq.RequestId,
		Timestamp: time.Now().Unix(),
		MessageType: &proto.TunnelMessage_Control{
			Control: &proto.TunnelControl{
				ControlType: &proto.TunnelControl_EstablishRequest{
					EstablishRequest: establishReq,
				},
				Message:   fmt.Sprintf("Requesting %s tunnel establishment", establishReq.TunnelType.String()),
				Timestamp: time.Now().Unix(),
			},
		},
	}

	// Send the control message to client
	if err := tunnelStream.Stream.Send(controlMsg); err != nil {
		return fmt.Errorf("failed to send tunnel establishment request: %w", err)
	}

	s.logger.Info("[ESTABLISH] Sent %s tunnel establishment request to client for domain: %s",
		establishReq.TunnelType.String(), domain)
	return nil
}
