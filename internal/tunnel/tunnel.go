package tunnel

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
	"strconv"
	"strings"
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

	// Set TCP keep-alive on tunnel connection
	if tcpConn, ok := t.conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetReadBuffer(32 * 1024)  // 32KB read buffer
		tcpConn.SetWriteBuffer(32 * 1024) // 32KB write buffer
	}

	for {
		select {
		case <-t.stopChan:
			t.logger.Info("[TUNNEL DEBUG] Received stop signal, stopping connection handler")
			return
		default:
			// Connect to local service for each request
			localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to connect to local service: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// Set TCP keep-alive on local connection
			if tcpConn, ok := localConn.(*net.TCPConn); ok {
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(30 * time.Second)
				tcpConn.SetReadBuffer(32 * 1024)  // 32KB read buffer
				tcpConn.SetWriteBuffer(32 * 1024) // 32KB write buffer
			}

			// Create buffered reader and writer for local connection
			localReader := bufio.NewReaderSize(localConn, 32*1024)
			localWriter := bufio.NewWriterSize(localConn, 32*1024)

			// Read request line
			requestLine, err := tunnelReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading request line: %v", err)
				}
				localConn.Close()
				if err == io.EOF {
					return // Exit if tunnel connection is closed
				}
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Request line: %s", strings.TrimSpace(requestLine))

			// Write request line to local service
			if _, err := localWriter.WriteString(requestLine); err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error writing request line: %v", err)
				localConn.Close()
				continue
			}

			// Read and forward headers
			var contentLength int64
			for {
				line, err := tunnelReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading header: %v", err)
					}
					localConn.Close()
					if err == io.EOF {
						return // Exit if tunnel connection is closed
					}
					break
				}

				// Write header line
				if _, err := localWriter.WriteString(line); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error writing header: %v", err)
					localConn.Close()
					break
				}

				// Parse Content-Length if present
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					contentLength, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")), 10, 64)
				}

				// Check for end of headers
				if line == "\r\n" {
					break
				}
			}

			// Flush headers
			if err := localWriter.Flush(); err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error flushing headers: %v", err)
				localConn.Close()
				continue
			}

			// Forward request body if present
			if contentLength > 0 {
				t.logger.Info("[TUNNEL DEBUG] Forwarding request body of length: %d", contentLength)
				written, err := io.CopyN(localWriter, tunnelReader, contentLength)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error forwarding request body: %v", err)
					localConn.Close()
					if err == io.EOF {
						return // Exit if tunnel connection is closed
					}
					continue
				}
				t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes of request body", written)

				// Flush body
				if err := localWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing request body: %v", err)
					localConn.Close()
					continue
				}
			}

			// Read response line
			responseLine, err := localReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading response line: %v", err)
				}
				localConn.Close()
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Response line: %s", strings.TrimSpace(responseLine))

			// Write response line to tunnel
			if _, err := tunnelWriter.WriteString(responseLine); err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error writing response line: %v", err)
				localConn.Close()
				continue
			}

			// Read and forward response headers
			contentLength = 0
			for {
				line, err := localReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading response header: %v", err)
					}
					localConn.Close()
					break
				}

				// Write header line
				if _, err := tunnelWriter.WriteString(line); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error writing response header: %v", err)
					localConn.Close()
					break
				}

				// Parse Content-Length if present
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					contentLength, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")), 10, 64)
				}

				// Check for end of headers
				if line == "\r\n" {
					break
				}
			}

			// Flush response headers
			if err := tunnelWriter.Flush(); err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error flushing response headers: %v", err)
				localConn.Close()
				continue
			}

			// Forward response body if present
			if contentLength > 0 {
				t.logger.Info("[TUNNEL DEBUG] Forwarding response body of length: %d", contentLength)
				written, err := io.CopyN(tunnelWriter, localReader, contentLength)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error forwarding response body: %v", err)
					localConn.Close()
					continue
				}
				t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes of response body", written)

				// Flush response body
				if err := tunnelWriter.Flush(); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error flushing response body: %v", err)
					localConn.Close()
					continue
				}
			}

			// Close local connection after handling request
			localConn.Close()
			t.logger.Info("[TUNNEL DEBUG] Request handling completed successfully")
		}
	}
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}