package tunnel

import (
	"crypto/tls"
	"fmt"
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

	// Store token for reconnection/handshake
	t.token = token

	// Connect to server with TLS
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Perform initial handshake
	if _, err := Perform(conn, token); err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	t.conn = conn
	t.wg.Add(1)

	// Start reading from connection to keep it alive
	go t.keepAlive()

	return nil
}

// Disconnect closes the tunnel connection
func (t *Tunnel) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return nil
	}

	// Signal stop
	close(t.stopChan)

	// Wait for keepAlive to finish
	t.wg.Wait()

	// Close connection
	if err := t.conn.Close(); err != nil {
		return fmt.Errorf("failed to close tunnel connection: %w", err)
	}

	t.conn = nil
	t.stopChan = make(chan struct{})

	return nil
}

// keepAlive reads from the connection to keep it alive and detect disconnections
func (t *Tunnel) keepAlive() {
	defer t.wg.Done()

	buffer := make([]byte, 1024)

	for {
		select {
		case <-t.stopChan:
			return
		default:
			// Just read and discard data to keep the connection alive
			if _, err := t.conn.Read(buffer); err != nil {
				if err != io.EOF {
					fmt.Printf("Connection error: %v\n", err)
				}
				return
			}
		}
	}
}