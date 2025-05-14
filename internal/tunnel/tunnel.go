package tunnel

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
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

	// Create channels for different message types
	dataChan := make(chan *TunnelMessage, 100)    // Buffer for data messages
	controlChan := make(chan *TunnelMessage, 100) // Buffer for control messages
	errChan := make(chan error, 2)                // Error channel
	t.stopChan = make(chan struct{})              // Stop channel

	// Create JSON encoder/decoder
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Start message reader goroutine
	go func() {
		defer close(dataChan)
		defer close(controlChan)

		for {
			select {
			case <-t.stopChan:
				return
			default:
				var msg TunnelMessage
				if err := decoder.Decode(&msg); err != nil {
					if err != io.EOF {
						errChan <- fmt.Errorf("error reading message: %w", err)
					}
					return
				}

				// Route message based on type
				switch msg.Type {
				case MessageTypePing, MessageTypePong:
					select {
					case controlChan <- &msg:
					default:
						t.logger.Warn("Control channel buffer full, dropping message")
					}
				case MessageTypeData:
					select {
					case dataChan <- &msg:
					default:
						t.logger.Warn("Data channel buffer full, dropping message")
					}
				default:
					t.logger.Error("Unknown message type: %s", msg.Type)
				}
			}
		}
	}()

	// Start ping handler goroutine
	go func() {
		for {
			select {
			case <-t.stopChan:
				return
			case msg := <-controlChan:
				if msg.Type == MessageTypePing {
					// Handle ping message
					var pingMsg PingMessage
					if err := json.Unmarshal(msg.Payload, &pingMsg); err != nil {
						t.logger.Error("Error unmarshaling ping: %v", err)
						continue
					}

					// Send pong response
					pongPayload, _ := json.Marshal(PongMessage{
						Timestamp: pingMsg.Timestamp,
						RTT:       time.Now().UnixNano() - pingMsg.Timestamp,
					})
					pongMsg := TunnelMessage{
						Type:    MessageTypePong,
						ID:      msg.ID, // Use same ID for correlation
						Payload: pongPayload,
					}

					// Set write deadline for pong
					conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := encoder.Encode(pongMsg); err != nil {
						errChan <- fmt.Errorf("error sending pong: %w", err)
						return
					}
					conn.SetWriteDeadline(time.Time{})
				}
			}
		}
	}()

	// Start data handler goroutine
	go func() {
		for {
			select {
			case <-t.stopChan:
				return
			case msg := <-dataChan:
				var dataPayload DataMessage
				if err := json.Unmarshal(msg.Payload, &dataPayload); err != nil {
					t.logger.Error("Error unmarshaling data message: %v", err)
					continue
				}

				// Connect to local service
				localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
				if err != nil {
					t.logger.Error("Failed to connect to local service: %v", err)
					continue
				}

				// Write data to local service
				if _, err := localConn.Write(dataPayload.Data); err != nil {
					t.logger.Error("Error writing to local service: %v", err)
					localConn.Close()
					continue
				}

				// Read response from local service
				response := make([]byte, 32*1024) // 32KB buffer
				n, err := localConn.Read(response)
				if err != nil && err != io.EOF {
					t.logger.Error("Error reading from local service: %v", err)
					localConn.Close()
					continue
				}

				// Send response back through tunnel
				responsePayload, _ := json.Marshal(DataMessage{
					Data: response[:n],
				})
				responseMsg := TunnelMessage{
					Type:    MessageTypeData,
					ID:      msg.ID, // Use same ID for correlation
					Payload: responsePayload,
				}

				// Set write deadline for response
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := encoder.Encode(responseMsg); err != nil {
					t.logger.Error("Error sending response: %v", err)
					localConn.Close()
					continue
				}
				conn.SetWriteDeadline(time.Time{})

				localConn.Close()
			}
		}
	}()

	// Wait for any error
	if err := <-errChan; err != nil {
		t.logger.Error("Connection error: %v", err)
		return err
	}

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
			// Create error channel for synchronization
			errChan := make(chan error, 2)
			requestDone := make(chan struct{})
			responseDone := make(chan struct{})

			// Connect to local service for each request
			localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", t.localPort), 5*time.Second)
			if err != nil {
				t.logger.Error("[TUNNEL DEBUG] Failed to connect to local service: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}
			defer localConn.Close()

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

			// Handle request forwarding in a goroutine
			go func() {
				defer close(requestDone)

				// Read request line
				requestLine, err := tunnelReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading request line: %v", err)
						errChan <- fmt.Errorf("error reading request line: %w", err)
					}
					return
				}
				t.logger.Info("[TUNNEL DEBUG] Request line: %s", strings.TrimSpace(requestLine))

				// Write request line to local service
				if _, err := localWriter.WriteString(requestLine); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error writing request line: %v", err)
					errChan <- fmt.Errorf("error writing request line: %w", err)
					return
				}

				// Read and forward headers
				var contentLength int64
				for {
					line, err := tunnelReader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							t.logger.Error("[TUNNEL DEBUG] Error reading header: %v", err)
							errChan <- fmt.Errorf("error reading header: %w", err)
						}
						return
					}

					// Write header line
					if _, err := localWriter.WriteString(line); err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error writing header: %v", err)
						errChan <- fmt.Errorf("error writing header: %w", err)
						return
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
					errChan <- fmt.Errorf("error flushing headers: %w", err)
					return
				}

				// Forward request body if present
				if contentLength > 0 {
					t.logger.Info("[TUNNEL DEBUG] Forwarding request body of length: %d", contentLength)
					written, err := io.CopyN(localWriter, tunnelReader, contentLength)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error forwarding request body: %v", err)
						errChan <- fmt.Errorf("error forwarding request body: %w", err)
						return
					}
					t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes of request body", written)

					// Flush body
					if err := localWriter.Flush(); err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error flushing body: %v", err)
						errChan <- fmt.Errorf("error flushing body: %w", err)
						return
					}
				}

				t.logger.Info("[TUNNEL DEBUG] Request forwarding completed")
			}()

			// Handle response forwarding in a goroutine
			go func() {
				defer close(responseDone)

				// Wait for request to be forwarded
				<-requestDone
				t.logger.Info("[TUNNEL DEBUG] Starting response handling")

				// Read response line
				responseLine, err := localReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.logger.Error("[TUNNEL DEBUG] Error reading response line: %v", err)
						errChan <- fmt.Errorf("error reading response line: %w", err)
					}
					return
				}
				t.logger.Info("[TUNNEL DEBUG] Response line: %s", strings.TrimSpace(responseLine))

				// Write response line to tunnel
				if _, err := tunnelWriter.WriteString(responseLine); err != nil {
					t.logger.Error("[TUNNEL DEBUG] Error writing response line: %v", err)
					errChan <- fmt.Errorf("error writing response line: %w", err)
					return
				}

				// Read and forward response headers
				var contentLength int64
				for {
					line, err := localReader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							t.logger.Error("[TUNNEL DEBUG] Error reading response header: %v", err)
							errChan <- fmt.Errorf("error reading response header: %w", err)
						}
						return
					}

					// Write header line
					if _, err := tunnelWriter.WriteString(line); err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error writing response header: %v", err)
						errChan <- fmt.Errorf("error writing response header: %w", err)
						return
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
					errChan <- fmt.Errorf("error flushing response headers: %w", err)
					return
				}

				// Forward response body if present
				if contentLength > 0 {
					t.logger.Info("[TUNNEL DEBUG] Forwarding response body of length: %d", contentLength)
					written, err := io.CopyN(tunnelWriter, localReader, contentLength)
					if err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error forwarding response body: %v", err)
						errChan <- fmt.Errorf("error forwarding response body: %w", err)
						return
					}
					t.logger.Info("[TUNNEL DEBUG] Wrote %d bytes of response body", written)

					// Flush response body
					if err := tunnelWriter.Flush(); err != nil {
						t.logger.Error("[TUNNEL DEBUG] Error flushing response body: %v", err)
						errChan <- fmt.Errorf("error flushing response body: %w", err)
						return
					}
				}

				t.logger.Info("[TUNNEL DEBUG] Response forwarding completed")
			}()

			// Wait for completion or error
			select {
			case err := <-errChan:
				t.logger.Error("[TUNNEL DEBUG] Connection error: %v", err)
			case <-responseDone:
				t.logger.Info("[TUNNEL DEBUG] Request/response cycle completed successfully")
			case <-time.After(30 * time.Second):
				t.logger.Error("[TUNNEL DEBUG] Connection timed out")
			}
		}
	}
}

// handlePingPong handles ping/pong messages from the server
func (t *Tunnel) handlePingPong() {
	t.logger.Info("[PING DEBUG] Starting ping/pong handler")
	defer t.logger.Info("[PING DEBUG] Ping/pong handler stopped")

	for {
		select {
		case <-t.stopChan:
			return
		default:
			// Wait for ping
			t.conn.SetReadDeadline(time.Now().Add(35 * time.Second)) // Slightly longer than server's ping interval
			var pingMsg PingMessage
			if err := json.NewDecoder(t.conn).Decode(&pingMsg); err != nil {
				if err != io.EOF {
					t.logger.Error("[PING DEBUG] Error reading ping: %v", err)
				}
				t.Disconnect()
				return
			}
			t.conn.SetReadDeadline(time.Time{}) // Reset deadline

			// Verify ping message
			if pingMsg.Type != "ping" {
				t.logger.Error("[PING DEBUG] Invalid ping message type: %s", pingMsg.Type)
				t.Disconnect()
				return
			}

			// Send pong response
			pongMsg := PongMessage{
				Type:      "pong",
				Timestamp: pingMsg.Timestamp,
				RTT:       time.Now().UnixNano() - pingMsg.Timestamp,
			}
			if err := json.NewEncoder(t.conn).Encode(pongMsg); err != nil {
				t.logger.Error("[PING DEBUG] Error sending pong: %v", err)
				t.Disconnect()
				return
			}

			t.logger.Debug("[PING DEBUG] Ping/pong successful, RTT: %v", time.Duration(pongMsg.RTT)*time.Nanosecond)
		}
	}
}

// IsConnected returns true if the tunnel is connected
func (t *Tunnel) IsConnected() bool {
	return t.conn != nil
}