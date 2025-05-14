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

			// Read the complete HTTP request
			var requestBuilder strings.Builder
			var contentLength int64
			var chunked bool
			var line string

			// Read headers
			for {
				line, err = tunnelReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading request headers: %v", err)
					} else {
						t.logger.Info("[TUNNEL DEBUG] Tunnel connection closed (EOF)")
					}
					localConn.Close()
					return
				}
				requestBuilder.WriteString(line)

				// Check for Content-Length
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					contentLength, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")), 10, 64)
				}

				// Check for chunked encoding
				if strings.HasPrefix(strings.ToLower(line), "transfer-encoding:") && strings.Contains(strings.ToLower(line), "chunked") {
					chunked = true
				}

				// End of headers
				if line == "\r\n" {
					break
				}
			}

			// Read body if present
			if contentLength > 0 {
				body := make([]byte, contentLength)
				_, err = io.ReadFull(tunnelReader, body)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading request body: %v", err)
					localConn.Close()
					return
				}
				requestBuilder.Write(body)
			} else if chunked {
				for {
					// Read chunk size
					line, err = tunnelReader.ReadString('\n')
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk size: %v", err)
						localConn.Close()
						return
					}
					requestBuilder.WriteString(line)

					// Parse chunk size
					chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error parsing chunk size: %v", err)
						localConn.Close()
						return
					}

					// End of chunks
					if chunkSize == 0 {
						// Read final CRLF
						line, err = tunnelReader.ReadString('\n')
						if err != nil {
							t.logger.Error("[TUNNEL DEBUG] Error reading final CRLF: %v", err)
							localConn.Close()
							return
						}
						requestBuilder.WriteString(line)
						break
					}

					// Read chunk data
					chunk := make([]byte, chunkSize)
					_, err = io.ReadFull(tunnelReader, chunk)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk data: %v", err)
						localConn.Close()
						return
					}
					requestBuilder.Write(chunk)

					// Read chunk CRLF
					line, err = tunnelReader.ReadString('\n')
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk CRLF: %v", err)
						localConn.Close()
						return
					}
					requestBuilder.WriteString(line)
				}
			}

			request := requestBuilder.String()
			t.logger.Info("[TUNNEL DEBUG] Complete request:\n%s", request)

			// Write the complete request to local service
			written, err := localWriter.WriteString(request)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error writing to local service: %v", err)
				localConn.Close()
				return
			}
			t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes to local service", written)

			if err := localWriter.Flush(); err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error flushing local writer: %v", err)
				localConn.Close()
				return
			}
			t.logger.Info("[TUNNEL DEBUG] Flushed local writer")

			// Read and forward the response
			var responseBuilder strings.Builder
			contentLength = 0
			chunked = false

			// Read response headers
			for {
				line, err = localReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading response headers: %v", err)
					}
					localConn.Close()
					return
				}
				responseBuilder.WriteString(line)

				// Check for Content-Length
				if strings.HasPrefix(strings.ToLower(line), "content-length:") {
					contentLength, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")), 10, 64)
				}

				// Check for chunked encoding
				if strings.HasPrefix(strings.ToLower(line), "transfer-encoding:") && strings.Contains(strings.ToLower(line), "chunked") {
					chunked = true
				}

				// End of headers
				if line == "\r\n" {
					break
				}
			}

			// Read response body if present
			if contentLength > 0 {
				body := make([]byte, contentLength)
				_, err = io.ReadFull(localReader, body)
				if err != nil && err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading response body: %v", err)
					localConn.Close()
					return
				}
				responseBuilder.Write(body)
			} else if chunked {
				for {
					// Read chunk size
					line, err = localReader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							t.logger.Error("[TUNNEL DEBUG] Error reading chunk size: %v", err)
						}
							localConn.Close()
							return
					}
					responseBuilder.WriteString(line)

					// Parse chunk size
					chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error parsing chunk size: %v", err)
						localConn.Close()
						return
					}

					// End of chunks
					if chunkSize == 0 {
						// Read final CRLF
						line, err = localReader.ReadString('\n')
						if err != nil && err != io.EOF {
							t.logger.Error("[TUNNEL DEBUG] Error reading final CRLF: %v", err)
							localConn.Close()
							return
						}
						responseBuilder.WriteString(line)
						break
					}

					// Read chunk data
					chunk := make([]byte, chunkSize)
					_, err = io.ReadFull(localReader, chunk)
					if err != nil && err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk data: %v", err)
						localConn.Close()
						return
					}
					responseBuilder.Write(chunk)

					// Read chunk CRLF
					line, err = localReader.ReadString('\n')
					if err != nil && err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk CRLF: %v", err)
						localConn.Close()
						return
					}
					responseBuilder.WriteString(line)
				}
			}

			response := responseBuilder.String()
			t.logger.Info("[TUNNEL DEBUG] Complete response:\n%s", response)

			// Write the complete response back to tunnel
			written, err = tunnelWriter.WriteString(response)
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