package tunnel

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
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
	conn     net.Conn
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	token    string
	yamuxSession *yamux.Session
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

	t.conn = conn

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

	// Read JSON header from stream
	reader := bufio.NewReader(stream)
	headerLine, err := reader.ReadBytes('\n')
	if err != nil {
		logger.Error("Failed to read stream header: %v", err)
		return
	}
	var header struct {
		Domain    string `json:"domain"`
		LocalPort int    `json:"local_port"`
		Protocol  string `json:"protocol"`
	}
	if err := json.Unmarshal(headerLine, &header); err != nil {
		logger.Error("Failed to parse stream header: %v", err)
		return
	}
	localAddr := fmt.Sprintf("localhost:%d", header.LocalPort)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		logger.Error("Failed to connect to local service at %s: %v", localAddr, err)
		return
	}
	defer localConn.Close()
	logger.Info("Proxying stream for domain %s between server and local service at %s", header.Domain, localAddr)
	// Bidirectional copy
	go io.Copy(localConn, reader)
	io.Copy(stream, localConn)
	logger.Info("Stream proxy finished for %s", localAddr)
}

// IsConnected returns true if the tunnel connection is active
func (t *Tunnel) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}