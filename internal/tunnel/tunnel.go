package tunnel

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
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
	t.logger.Info("[TUNNEL DEBUG] Starting to handle incoming connections")

	// Create buffered reader and writer for the tunnel connection
	tunnelReader := bufio.NewReader(t.conn)
	tunnelWriter := bufio.NewWriter(t.conn)

	for {
		select {
		case <-t.stopChan:
			t.logger.Info("[TUNNEL DEBUG] Received stop signal, stopping connection handler")
			return
		default:
			// Create a new connection for each request
			t.logger.Info("[TUNNEL DEBUG] Attempting to connect to local service at localhost:%d", t.localPort)
			localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to connect to local service: %v", err)
				time.Sleep(1 * time.Second) // Wait before retrying
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Successfully connected to local service")

			// Create buffered reader and writer for the local connection
			localReader := bufio.NewReader(localConn)
			localWriter := bufio.NewWriter(localConn)

			// Read the request from tunnel
			requestData := make([]byte, 32*1024)
			n, err := tunnelReader.Read(requestData)
			if err != nil {
				if err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading request from tunnel: %v", err)
				} else {
					t.logger.Info("[TUNNEL DEBUG] Tunnel connection closed (EOF)")
				}
				localConn.Close()
				return
			}

			if n > 0 {
				t.logger.Info("[TUNNEL DEBUG] Read %d bytes from tunnel", n)
				t.logger.Info("[TUNNEL DEBUG] Request from tunnel: %s", string(requestData[:n]))

				// Write request to local service
				written, err := localWriter.Write(requestData[:n])
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error writing to local service: %v", err)
					localConn.Close()
					continue
				}
				t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes to local service", written)

				if err := localWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing local writer: %v", err)
					localConn.Close()
					continue
				}
				t.logger.Info("[TUNNEL DEBUG] Flushed local writer")

				// Read response from local service
				responseData := make([]byte, 32*1024)
				n, err = localReader.Read(responseData)
				if err != nil && err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading from local service: %v", err)
					localConn.Close()
					continue
				}

				if n > 0 {
					t.logger.Info("[TUNNEL DEBUG] Read %d bytes from local service", n)
					t.logger.Info("[TUNNEL DEBUG] Response from local service: %s", string(responseData[:n]))

					// Write response back to tunnel
					written, err = tunnelWriter.Write(responseData[:n])
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error writing to tunnel: %v", err)
						localConn.Close()
						return
					}
					t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes to tunnel", written)

					if err := tunnelWriter.Flush(); err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error flushing tunnel writer: %v", err)
						localConn.Close()
						return
					}
					t.logger.Info("[TUNNEL DEBUG] Flushed tunnel writer")
				}
			}

			// Close the local connection after handling the request/response
			localConn.Close()
			t.logger.Info("[TUNNEL DEBUG] Connection handling completed")
		}
	}
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}