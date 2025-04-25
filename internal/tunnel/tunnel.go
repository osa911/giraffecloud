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
	local    net.Conn
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewTunnel creates a new tunnel instance
func NewTunnel(localAddr string) (*Tunnel, error) {
	// Connect to local service
	local, err := net.Dial("tcp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local service: %w", err)
	}

	return &Tunnel{
		local:    local,
		stopChan: make(chan struct{}),
	}, nil
}

// Connect establishes a tunnel connection to the server
func (t *Tunnel) Connect(serverAddr string, token string, tlsConfig *tls.Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Connect to server with TLS
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Perform handshake
	if _, err := Perform(conn, token); err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	t.conn = conn
	t.wg.Add(2)

	// Start bidirectional forwarding
	go t.forward(t.conn, t.local)
	go t.forward(t.local, t.conn)

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

	// Wait for forwarders to finish
	t.wg.Wait()

	// Close connections
	if err := t.conn.Close(); err != nil {
		return fmt.Errorf("failed to close tunnel connection: %w", err)
	}
	if err := t.local.Close(); err != nil {
		return fmt.Errorf("failed to close local connection: %w", err)
	}

	t.conn = nil
	t.local = nil
	t.stopChan = make(chan struct{})

	return nil
}

// forward copies data from src to dst until either EOF is reached or an error occurs
func (t *Tunnel) forward(dst, src net.Conn) {
	defer t.wg.Done()

	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		select {
		case <-t.stopChan:
			return
		default:
			n, err := src.Read(buffer)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading from connection: %v\n", err)
				}
				return
			}

			if n > 0 {
				_, err := dst.Write(buffer[:n])
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					return
				}
			}
		}
	}
}