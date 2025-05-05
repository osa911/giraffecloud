package tunnel

import (
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
	"sync"
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

// Connect establishes a tunnel connection to the server
func (t *Tunnel) Connect(serverAddr string, token string, tlsConfig *tls.Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger := logging.GetGlobalLogger()
	logger.Info("Attempting to connect to server at %s", serverAddr)

	// Store token for reconnection/handshake
	t.token = token

	// Connect to server with TLS
	logger.Info("Establishing TLS connection...")
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		logger.Error("Failed to establish TLS connection: %v", err)
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
	t.wg.Add(1)

	// Start reading from connection to keep it alive
	logger.Info("Starting connection keep-alive routine")
	go t.keepAlive()

	logger.Info("Tunnel connection established successfully")
	return nil
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

	// Signal stop
	close(t.stopChan)
	logger.Info("Stop signal sent to keep-alive routine")

	// Wait for keepAlive to finish
	t.wg.Wait()
	logger.Info("Keep-alive routine stopped")

	// Close connection
	if err := t.conn.Close(); err != nil {
		logger.Error("Failed to close tunnel connection: %v", err)
		return fmt.Errorf("failed to close tunnel connection: %w", err)
	}
	logger.Info("Connection closed successfully")

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
					logger.Error("Connection error in keep-alive routine: %v", err)
				} else {
					logger.Info("Connection closed by server (EOF)")
				}
				return
			}
		}
	}
}