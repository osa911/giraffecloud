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
			// First, read the request line
			requestLine, err := tunnelReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					t.logger.Error("[TUNNEL DEBUG] Error reading request line: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Request line: %s", strings.TrimSpace(requestLine))

			// Initialize request data with the request line
			requestData := []byte(requestLine)

			// Read headers until we hit an empty line
			for {
				line, err := tunnelReader.ReadString('\n')
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading header line: %v", err)
					continue
				}
				requestData = append(requestData, []byte(line)...)

				// Check if we've reached the end of headers
				if line == "\r\n" {
					break
				}
			}

			t.logger.Info("[TUNNEL DEBUG] Read request headers (%d bytes)", len(requestData))

			// Parse the request line to get the method
			parts := strings.Split(strings.TrimSpace(requestLine), " ")
			if len(parts) < 3 {
				t.logger.Error("[TUNNEL DEBUG] Invalid request line format")
				continue
			}
			method := parts[0]

			// Check for Content-Length in headers
			contentLength := 0
			headers := string(requestData)
			if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
				contentLength, _ = strconv.Atoi(match[1])
			}

			// Read body for POST/PUT/PATCH methods or if Content-Length is present
			if (method == "POST" || method == "PUT" || method == "PATCH" || contentLength > 0) && !strings.Contains(headers, "Transfer-Encoding: chunked") {
				t.logger.Info("[TUNNEL DEBUG] Reading request body of length: %d", contentLength)
				body := make([]byte, contentLength)
				_, err := io.ReadFull(tunnelReader, body)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading request body: %v", err)
					continue
				}
				requestData = append(requestData, body...)
			} else if strings.Contains(headers, "Transfer-Encoding: chunked") {
				t.logger.Info("[TUNNEL DEBUG] Reading chunked request body")
				for {
					// Read chunk size line
					line, err := tunnelReader.ReadString('\n')
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk size: %v", err)
						continue
					}
					requestData = append(requestData, []byte(line)...)

					// Parse chunk size
					chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
					if err != nil || chunkSize == 0 {
						break
					}

					// Read chunk data
					chunk := make([]byte, chunkSize+2) // +2 for CRLF
					_, err = io.ReadFull(tunnelReader, chunk)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk data: %v", err)
						break
					}
					requestData = append(requestData, chunk...)
				}
				// Add final CRLF
				requestData = append(requestData, []byte("\r\n")...)
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

			// Read the response status line
			statusLine, err := localReader.ReadString('\n')
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Error reading response status line: %v", err)
				localConn.Close()
				continue
			}
			t.logger.Info("[TUNNEL DEBUG] Response status: %s", strings.TrimSpace(statusLine))

			// Initialize response data with status line
			responseData := []byte(statusLine)

			// Read response headers
			for {
				line, err := localReader.ReadString('\n')
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading response header: %v", err)
					localConn.Close()
					continue
				}
				responseData = append(responseData, []byte(line)...)

				// Check if we've reached the end of headers
				if line == "\r\n" {
					break
				}
			}

			// Check for Content-Length and Transfer-Encoding in response
			headers = string(responseData)
			contentLength = 0
			if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
				contentLength, _ = strconv.Atoi(match[1])
			}

			// Read response body based on Content-Length or Transfer-Encoding
			if contentLength > 0 {
				t.logger.Info("[TUNNEL DEBUG] Reading response body of length: %d", contentLength)
				body := make([]byte, contentLength)
				_, err := io.ReadFull(localReader, body)
				if err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error reading response body: %v", err)
					localConn.Close()
					continue
				}
				responseData = append(responseData, body...)
			} else if strings.Contains(headers, "Transfer-Encoding: chunked") {
				t.logger.Info("[TUNNEL DEBUG] Reading chunked response body")
				for {
					// Read chunk size
					line, err := localReader.ReadString('\n')
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error reading chunk size: %v", err)
						break
					}
					responseData = append(responseData, []byte(line)...)

					// Parse chunk size
					chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
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
				// Add final CRLF
				responseData = append(responseData, []byte("\r\n")...)
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