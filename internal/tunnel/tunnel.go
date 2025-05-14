package tunnel

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
	"regexp"
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

	for {
		select {
		case <-t.stopChan:
			t.logger.Info("[TUNNEL DEBUG] Received stop signal, stopping connection handler")
			return
		default:
			// Read the request line and headers
			var requestData []byte
			for {
				line, err := tunnelReader.ReadBytes('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading request line: %v", err)
					}
					time.Sleep(100 * time.Millisecond)
					continue
				}
				requestData = append(requestData, line...)

				// Check if we've reached the end of headers
				if len(line) == 2 && line[0] == '\r' && line[1] == '\n' {
					break
				}
			}

			if len(requestData) == 0 {
				t.logger.Info("[TUNNEL DEBUG] No request data received, continuing...")
				continue
			}

			t.logger.Info("[TUNNEL DEBUG] Read request headers (%d bytes)", len(requestData))

			// Read request body if Content-Length is present
			contentLength := 0
			headers := string(requestData)
			if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
				contentLength, _ = strconv.Atoi(match[1])
			}

			if contentLength > 0 {
				t.logger.Info("[TUNNEL DEBUG] Reading request body of length: %d", contentLength)
				body := make([]byte, contentLength)
				_, err := io.ReadFull(tunnelReader, body)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading request body: %v", err)
					continue
				}
				requestData = append(requestData, body...)
			}

			t.logger.Info("[TUNNEL DEBUG] Total request size: %d bytes", len(requestData))

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
			localReader := bufio.NewReaderSize(localConn, 32*1024)
			localWriter := bufio.NewWriterSize(localConn, 32*1024)

			// Write the request to the local service
			_, err = localWriter.Write(requestData)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to write request to local service: %v", err)
				localConn.Close()
				continue
			}
			err = localWriter.Flush()
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to flush request to local service: %v", err)
				localConn.Close()
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Wrote request to local service")

			// Read the response headers
			var responseData []byte
			for {
				line, err := localReader.ReadBytes('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading response headers: %v", err)
					}
					break
				}
				responseData = append(responseData, line...)

				// Check if we've reached the end of headers
				if len(line) == 2 && line[0] == '\r' && line[1] == '\n' {
					break
				}
			}

			// Read response body based on Content-Length or Transfer-Encoding
			headers = string(responseData)
			contentLength = 0
			if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
				contentLength, _ = strconv.Atoi(match[1])
			}
			chunked := regexp.MustCompile(`(?i)Transfer-Encoding: chunked`).MatchString(headers)

			if contentLength > 0 {
				t.logger.Info("[TUNNEL DEBUG] Reading response body of length: %d", contentLength)
				body := make([]byte, contentLength)
				_, err := io.ReadFull(localReader, body)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading response body: %v", err)
				} else {
					responseData = append(responseData, body...)
				}
			} else if chunked {
				t.logger.Info("[TUNNEL DEBUG] Reading chunked response body")
				for {
					// Read chunk size
					line, err := localReader.ReadBytes('\n')
					if err != nil {
						if err != io.EOF {
							t.logger.Error("[TUNNEL DEBUG] Error reading chunk size: %v", err)
						}
						break
					}
					responseData = append(responseData, line...)

					// Parse chunk size
					chunkSize, err := strconv.ParseInt(strings.TrimSpace(string(line)), 16, 64)
					if err != nil || chunkSize == 0 {
						break
					}

					// Read chunk data
					chunk := make([]byte, chunkSize+2) // +2 for CRLF
					_, err = io.ReadFull(localReader, chunk)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk data: %v", err)
						break
					}
					responseData = append(responseData, chunk...)
				}
			}

			if len(responseData) == 0 {
				t.logger.Error("[TUNNEL DEBUG] No response data received from local service")
				localConn.Close()
				continue
			}

			t.logger.Info("[TUNNEL DEBUG] Total response size: %d bytes", len(responseData))

			// Write the response back through the tunnel
			_, err = tunnelWriter.Write(responseData)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to write response to tunnel: %v", err)
			} else {
				err = tunnelWriter.Flush()
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Failed to flush response to tunnel: %v", err)
				} else {
					t.logger.Info("[TUNNEL DEBUG] Successfully wrote and flushed response")
				}
			}

			// Cleanup
			localConn.Close()
			t.logger.Info("[TUNNEL DEBUG] Connection handling completed")
		}
	}
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}