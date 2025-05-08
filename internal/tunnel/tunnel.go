package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Tunnel represents a secure tunnel connection
type Tunnel struct {
	conn        net.Conn
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	token       string
	yamuxSession *yamux.Session
	Domain      string
	LocalPort   int
}

// NewTunnel creates a new tunnel instance
func NewTunnel() *Tunnel {
	return &Tunnel{
		stopChan: make(chan struct{}),
	}
}

// dialTLSWithRetry tries to connect to the server with TLS, with retries and exponential backoff
func dialTLSWithRetry(ctx context.Context, network, address string, config *tls.Config, maxAttempts int, baseDelay time.Duration, logger *logging.Logger) (net.Conn, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := tls.Dial(network, address, config)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if logger != nil {
			logger.Warn("[RETRY] Attempt %d/%d: failed to connect to %s: %v", attempt, maxAttempts, address, err)
		}
		delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("all %d attempts failed to connect to %s: %w", maxAttempts, address, lastErr)
}

// ConnectWithContext establishes a tunnel connection to the server using the provided context
func (t *Tunnel) ConnectWithContext(ctx context.Context, serverAddr string, token string, tlsConfig *tls.Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger := logging.GetGlobalLogger()
	logger.Info("Attempting to connect to server at %s", serverAddr)

	// Store token for reconnection/handshake
	t.token = token

	// Connect to server with TLS and retry logic
	logger.Info("Establishing TLS connection...")

	conn, err := dialTLSWithRetry(ctx, "tcp", serverAddr, tlsConfig, 5, 2*time.Second, logger)
	if err != nil {
		logger.Error("Failed to establish TLS connection after retries: %v", err)
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	logger.Info("TLS connection established successfully")

	// Perform initial handshake
	logger.Info("Starting handshake process...")
	resp, err := Perform(conn, token)
	if err != nil {
		conn.Close()
		logger.Error("Handshake failed: %v", err)
		return fmt.Errorf("handshake failed: %w", err)
	}
	logger.Info("Handshake completed successfully: %s", resp.Message)
	logger.Info("Domain: %s, LocalPort: %d", resp.Domain, resp.LocalPort)

	t.conn = conn

	// Set Domain and LocalPort from handshake response
	t.Domain = resp.Domain
	t.LocalPort = resp.LocalPort

	logger.Info("Proxying traffic between this client and server at %s (tunnel established)", serverAddr)

	// Create yamux session
	session, err := yamux.Client(t.conn, nil)
	if err != nil {
		logger.Error("Failed to create yamux session: %v", err)
		return fmt.Errorf("failed to create yamux session: %w", err)
	}
	t.yamuxSession = session

	// Start accepting streams from the server
	go t.acceptStreams()

	logger.Info("Tunnel connection established successfully (yamux multiplexing enabled)")
	return nil
}

// Connect establishes a tunnel connection to the server (with 30s timeout)
func (t *Tunnel) Connect(serverAddr string, token string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return t.ConnectWithContext(ctx, serverAddr, token, tlsConfig)
}

// Disconnect closes the tunnel connection
func (t *Tunnel) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger := logging.GetGlobalLogger()

	if t.conn == nil {
		logger.Info("No active connection to disconnect")
		return nil
	}

	logger.Info("Initiating tunnel disconnect...")

	// Close connection first to unblock acceptStreams
	if err := t.conn.Close(); err != nil {
		logger.Error("Failed to close tunnel connection: %v", err)
		return fmt.Errorf("failed to close tunnel connection: %w", err)
	}
	logger.Info("Connection closed successfully (before signaling stop)")

	// Signal stop
	close(t.stopChan)
	logger.Info("Stop signal sent to acceptStreams routine")

	// Wait for acceptStreams to finish
	t.wg.Wait()
	logger.Info("AcceptStreams routine stopped")

	t.conn = nil
	t.stopChan = make(chan struct{})

	logger.Info("Tunnel disconnected successfully")
	return nil
}

// acceptStreams listens for new streams from the server and proxies them to the local service
func (t *Tunnel) acceptStreams() {
	logger := logging.GetGlobalLogger()
	cfg, err := LoadConfig()
	if err != nil {
		logger.Error("Failed to load config in acceptStreams: %v", err)
		return
	}
	for {
		stream, err := t.yamuxSession.Accept()
		if err != nil {
			logger.Error("Failed to accept yamux stream: %v", err)
			return
		}
		go t.handleStream(stream, cfg)
	}
}

// handleStream proxies a single yamux stream to the local service
func (t *Tunnel) handleStream(stream net.Conn, cfg *Config) {
	logger := logging.GetGlobalLogger()
	defer stream.Close()

	reader := bufio.NewReader(stream)
	// Use the LocalPort from the handshake response instead of static config
	localAddr := fmt.Sprintf("localhost:%d", t.LocalPort)
	logger.Info("LocalAddr: %s", localAddr)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		logger.Error("Failed to connect to local service at %s: %v", localAddr, err)
		return
	}
	logger.Info("Connected to local service at %s", localAddr)
	defer localConn.Close()
	logger.Info("Proxying stream between server and local service at %s", localAddr)

	// Copy any buffered data, peek and log for debugging partial/incomplete HTTP requests
	if buffered := reader.Buffered(); buffered > 0 {
		logger.Info("Forwarding %d bytes of buffered data to local service", buffered)
		peek, err := reader.Peek(buffered)
		if err == nil {
			loggedBytes := 512
			if len(peek) < 512 {
				loggedBytes = len(peek)
			}
			logger.Info("Buffered peek data (first %d bytes): %q", loggedBytes, peek[:loggedBytes])
			_, writeErr := localConn.Write(peek)
			if writeErr != nil {
				logger.Error("Failed to write peeked data to localConn: %v", writeErr)
			} else {
				_, _ = reader.Discard(buffered)
			}
		} else {
			logger.Error("Failed to peek buffered data: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logger.Info("Starting io.Copy from stream to localConn (server->local)")
		n, err := io.Copy(localConn, reader)
		logger.Info("Copied %d bytes from stream to localConn (server->local), err=%v", n, err)
		if tcp, ok := localConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("Starting io.Copy from localConn to stream (local->server)")
		n, err := io.Copy(stream, localConn)
		logger.Info("Copied %d bytes from localConn to stream (local->server), err=%v", n, err)
		if tcp, ok := stream.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	}()

	wg.Wait()
	logger.Info("Stream proxy finished for %s", localAddr)
}

// IsConnected returns true if the tunnel connection is active
func (t *Tunnel) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}