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
	tunnelReader := bufio.NewReaderSize(t.conn, 32*1024) // 32KB buffer
	tunnelWriter := bufio.NewWriterSize(t.conn, 32*1024)

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

			// Set timeouts for the local connection
			localConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			localConn.SetWriteDeadline(time.Now().Add(30 * time.Second))

			// Create buffered reader and writer for the local connection
			localReader := bufio.NewReaderSize(localConn, 32*1024)
			localWriter := bufio.NewWriterSize(localConn, 32*1024)

			// Create error channels for both directions
			tunnelToLocalErr := make(chan error, 1)
			localToTunnelErr := make(chan error, 1)
			done := make(chan struct{})

			// Forward data in both directions concurrently
			go func() {
				n, err := io.Copy(localWriter, tunnelReader)
				t.logger.Info("[TUNNEL DEBUG] Forwarded %d bytes from tunnel to local", n)
				if err := localWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing local writer: %v", err)
				} else {
					t.logger.Info("[TUNNEL DEBUG] Successfully flushed local writer")
				}
				tunnelToLocalErr <- err
			}()

			go func() {
				n, err := io.Copy(tunnelWriter, localReader)
				t.logger.Info("[TUNNEL DEBUG] Forwarded %d bytes from local to tunnel", n)
				if err := tunnelWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing tunnel writer: %v", err)
				} else {
					t.logger.Info("[TUNNEL DEBUG] Successfully flushed tunnel writer")
				}
				localToTunnelErr <- err
			}()

			// Set up a timeout for the entire operation
			go func() {
				select {
				case <-time.After(60 * time.Second):
					t.logger.Info("[TUNNEL DEBUG] Connection timeout")
					close(done)
				case <-done:
					return
				}
			}()

			cleanup := func() {
				t.logger.Info("[TUNNEL DEBUG] Starting cleanup")
				// Ensure all buffered data is written before closing
				if err := localWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing local writer during cleanup: %v", err)
				}
				if err := tunnelWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing tunnel writer during cleanup: %v", err)
				}

				// Reset deadlines
				localConn.SetReadDeadline(time.Time{})
				localConn.SetWriteDeadline(time.Time{})

				// Close the local connection
				if err := localConn.Close(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error closing local connection: %v", err)
				}

				close(done)
				t.logger.Info("[TUNNEL DEBUG] Cleanup completed")
			}

			// Wait for data transfer to complete in both directions
			var tunnelToLocalError, localToTunnelError error
			select {
			case tunnelToLocalError = <-tunnelToLocalErr:
				t.logger.Info("[TUNNEL DEBUG] Tunnel to local transfer completed")
				// Wait a short time for response data
				select {
				case localToTunnelError = <-localToTunnelErr:
					t.logger.Info("[TUNNEL DEBUG] Local to tunnel transfer completed")
				case <-time.After(5 * time.Second):
					t.logger.Info("[TUNNEL DEBUG] Waiting for response timed out")
				}
			case localToTunnelError = <-localToTunnelErr:
				t.logger.Info("[TUNNEL DEBUG] Local to tunnel transfer completed first")
				// Wait for request to complete
				select {
				case tunnelToLocalError = <-tunnelToLocalErr:
					t.logger.Info("[TUNNEL DEBUG] Tunnel to local transfer completed")
				case <-time.After(5 * time.Second):
					t.logger.Info("[TUNNEL DEBUG] Waiting for request completion timed out")
				}
			case <-t.stopChan:
				t.logger.Info("[TUNNEL DEBUG] Received stop signal during transfer")
				cleanup()
				return
			case <-done:
				t.logger.Info("[TUNNEL DEBUG] Connection timed out")
			}

			// Log any non-EOF errors
			if tunnelToLocalError != nil && tunnelToLocalError != io.EOF {
				t.logger.Error("[TUNNEL DEBUG] Error forwarding tunnel to local: %v", tunnelToLocalError)
			}
			if localToTunnelError != nil && localToTunnelError != io.EOF {
				t.logger.Error("[TUNNEL DEBUG] Error forwarding local to tunnel: %v", localToTunnelError)
			}

			cleanup()
			t.logger.Info("[TUNNEL DEBUG] Connection handling completed")
		}
	}
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}