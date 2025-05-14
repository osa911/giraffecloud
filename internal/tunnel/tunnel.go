package tunnel

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
	"sync"
	"time"
)

// Tunnel represents a secure tunnel connection
type Tunnel struct {
	conn      net.Conn
	stopChan  chan struct{}
	token     string
	domain    string
	localPort int
	logger    *logging.Logger
}

// NewTunnel creates a new tunnel instance
func NewTunnel() *Tunnel {
	return &Tunnel{
		stopChan: make(chan struct{}),
		logger:   logging.GetGlobalLogger(),
	}
}

// Connect establishes a tunnel connection to the server
func (t *Tunnel) Connect(serverAddr, token, domain string, localPort int, tlsConfig *tls.Config) error {
	t.token = token
	t.domain = domain
	t.localPort = localPort

	// Connect to server with TLS
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	t.conn = conn

	// Perform handshake
	resp, err := Perform(conn, token)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	// Update local values with server response
	t.domain = resp.Domain
	if t.localPort <= 0 {
		t.localPort = resp.TargetPort
	}

	// Check if the local port is actually listening
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
	if err != nil {
		conn.Close()
		return fmt.Errorf("no service found listening on port %d - make sure your service is running first", t.localPort)
	}
	localConn.Close()

	// Start handling incoming connections
	go t.handleIncomingConnections()

	return nil
}

// Disconnect closes the tunnel connection
func (t *Tunnel) Disconnect() error {
	if t.conn == nil {
		return nil
	}

	close(t.stopChan)
	return t.conn.Close()
}

// handleIncomingConnections handles incoming connections from the server
func (t *Tunnel) handleIncomingConnections() {
	// Connect to local service once
	localConn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", t.localPort))
	if err != nil {
		t.logger.Error("Failed to connect to local service: %v", err)
		return
	}
	defer localConn.Close()

	// Create buffered reader and writer for the tunnel connection
	tunnelReader := bufio.NewReader(t.conn)
	tunnelWriter := bufio.NewWriter(t.conn)
	localReader := bufio.NewReader(localConn)
	localWriter := bufio.NewWriter(localConn)

	// Copy data bidirectionally with buffering
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy from tunnel to local service
	go func() {
		defer wg.Done()
		for {
			// Read the raw request from tunnel
			data := make([]byte, 4096)
			n, err := tunnelReader.Read(data)
			if err != nil {
				if err != io.EOF {
					t.logger.Error("Error reading from tunnel: %v", err)
				}
				return
			}

			// Write to local service
			_, err = localWriter.Write(data[:n])
			if err != nil {
				t.logger.Error("Error writing to local service: %v", err)
				return
			}
			localWriter.Flush()
		}
	}()

	// Copy from local service to tunnel
	go func() {
		defer wg.Done()
		for {
			// Read response from local service
			data := make([]byte, 4096)
			n, err := localReader.Read(data)
			if err != nil {
				if err != io.EOF {
					t.logger.Error("Error reading from local service: %v", err)
				}
				return
			}

			// Write back to tunnel
			_, err = tunnelWriter.Write(data[:n])
			if err != nil {
				t.logger.Error("Error writing to tunnel: %v", err)
				return
			}
			tunnelWriter.Flush()
		}
	}()

	// Wait for either connection to close
	wg.Wait()
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}