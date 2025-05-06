package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

// Tunnel represents a secure tunnel connection
type Tunnel struct {
	conn     net.Conn
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	token    string
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

	t.wg.Add(1)

	// Start reading from connection to keep it alive
	logger.Info("Starting connection keep-alive routine")
	go t.keepAlive()

	logger.Info("Tunnel connection established successfully")
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

	// Close connection first to unblock keepAlive
	if err := t.conn.Close(); err != nil {
		logger.Error("Failed to close tunnel connection: %v", err)
		return fmt.Errorf("failed to close tunnel connection: %w", err)
	}
	logger.Info("Connection closed successfully (before signaling stop)")

	// Signal stop
	close(t.stopChan)
	logger.Info("Stop signal sent to keep-alive routine")

	// Wait for keepAlive to finish
	t.wg.Wait()
	logger.Info("Keep-alive routine stopped")

	t.conn = nil
	t.stopChan = make(chan struct{})

	logger.Info("Tunnel disconnected successfully")
	return nil
}

// keepAlive reads from the connection to keep it alive and detect disconnections
func (t *Tunnel) keepAlive() {
	defer t.wg.Done()

	logger := logging.GetGlobalLogger()
	logger.Info("Keep-alive routine started")

	buffer := make([]byte, 1024)

	for {
		select {
		case <-t.stopChan:
			logger.Info("Keep-alive routine received stop signal")
			return
		default:
			// Just read and discard data to keep the connection alive
			if _, err := t.conn.Read(buffer); err != nil {
				if err != io.EOF {
					if strings.Contains(err.Error(), "use of closed network connection") {
						logger.Info("Connection closed (expected on shutdown): %v", err)
					} else {
						logger.Error("Connection error in keep-alive routine: %v", err)
					}
				} else {
					logger.Info("Connection closed by server (EOF)")
				}
				return
			}
		}
	}
}

// IsConnected returns true if the tunnel connection is active
func (t *Tunnel) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}