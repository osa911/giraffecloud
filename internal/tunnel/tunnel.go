package tunnel

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"net"
	"net/http"
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

	// Simplify TLS config - use defaults for better compatibility
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true, // Only for development
		}
	}

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

	t.logger.Info("Tunnel connected successfully. Domain: %s, Local Port: %d", t.domain, t.localPort)

	// Start HTTP forwarding
	go t.handleConnection()

	return nil
}

// handleConnection handles HTTP requests from the tunnel server
func (t *Tunnel) handleConnection() {
	defer func() {
		if t.conn != nil {
			t.conn.Close()
		}
		t.logger.Info("Tunnel connection closed")
	}()

	t.logger.Info("Starting HTTP forwarding for tunnel connection")

	// Create buffered reader for parsing HTTP
	tunnelReader := bufio.NewReader(t.conn)

	// Handle incoming HTTP requests from the tunnel
	for {
		select {
		case <-t.stopChan:
			return
		default:
			// Set read timeout for incoming requests
			t.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Parse the HTTP request using Go's built-in HTTP parser
			request, err := http.ReadRequest(tunnelReader)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is normal, continue
					continue
				}
				// Connection closed or error
				t.logger.Info("Tunnel connection closed: %v", err)
				return
			}

			// We have an HTTP request, reset deadline
			t.conn.SetReadDeadline(time.Time{})

			t.logger.Info("Received HTTP request: %s %s", request.Method, request.URL.Path)

			// Connect to local service for this request
			localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
			if err != nil {
				t.logger.Error("Failed to connect to local service: %v", err)
				// Send error response back through tunnel
				errorResponse := "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
				t.conn.Write([]byte(errorResponse))
				continue
			}

			// Forward the request to local service
			if err := request.Write(localConn); err != nil {
				t.logger.Error("Failed to write request to local service: %v", err)
				localConn.Close()
				continue
			}

			t.logger.Info("Request forwarded to local service, reading response")

			// Read response from local service
			localReader := bufio.NewReader(localConn)
			response, err := http.ReadResponse(localReader, request)
			if err != nil {
				t.logger.Error("Error reading response from local service: %v", err)
				localConn.Close()
				continue
			}

			// Write response back to tunnel
			if err := response.Write(t.conn); err != nil {
				t.logger.Error("Error writing response to tunnel: %v", err)
				localConn.Close()
				continue
			}

			localConn.Close()
			t.logger.Info("HTTP request/response cycle completed")
		}
	}
}

// Disconnect closes the tunnel connection and cleans up resources
func (t *Tunnel) Disconnect() error {
	if t.conn == nil {
		return nil
	}

	// Signal all goroutines to stop
	close(t.stopChan)

	// Set a deadline for graceful shutdown
	t.conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send a final control message to notify server (simplified)
	encoder := json.NewEncoder(t.conn)
	closeMsg := map[string]string{
		"type":   "control",
		"action": "shutdown",
		"reason": "client_disconnect",
	}
	_ = encoder.Encode(closeMsg)

	// Close the connection
	err := t.conn.Close()
	t.conn = nil

	// Wait for a moment to allow goroutines to clean up
	time.Sleep(100 * time.Millisecond)

	t.logger.Info("Tunnel disconnected")
	return err
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}