package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"io"
	"net"
	"time"
)

/**
	FYI: We use this server to handle the handshake and proxy the connection to the local service

TODO:
- Add rate limiting
- Add logging
- Add metrics
*/

// ClientIPUpdateFunc is a callback function for client IP updates
type ClientIPUpdateFunc func(ctx context.Context, tunnelID uint32, clientIP string) error

// TunnelServer represents the tunnel server
type TunnelServer struct {
	listener      net.Listener
	logger        *logging.Logger
	tlsConfig     *tls.Config
	tokenRepo     repository.TokenRepository
	tunnelRepo    repository.TunnelRepository
	tunnelService interfaces.TunnelService
	connections   *ConnectionManager
}


// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository, tunnelService interfaces.TunnelService) *TunnelServer {
	return &TunnelServer{
		logger:        logging.GetGlobalLogger(),
		connections:   NewConnectionManager(),
		tlsConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
		},
		tokenRepo:     tokenRepo,
		tunnelRepo:    tunnelRepo,
		tunnelService: tunnelService,
	}
}

// Start starts the tunnel server
func (s *TunnelServer) Start(addr string) error {
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.listener = tls.NewListener(tcpListener, s.tlsConfig)
	go s.acceptConnections()

	s.logger.Info("Tunnel server listening on %s", addr)
	return nil
}

// Stop stops the tunnel server
func (s *TunnelServer) Stop() error {
	if s.listener == nil {
		return nil
	}

	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	return nil
}

func (s *TunnelServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.logger.Error("Failed to accept connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles a new tunnel connection
func (s *TunnelServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Create JSON encoder/decoder
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Read handshake request
	var req TunnelHandshakeRequest
	if err := decoder.Decode(&req); err != nil {
		s.logger.Error("Failed to decode handshake: %v", err)
		return
	}

	// Find user by API token
	token, err := s.tokenRepo.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to authenticate: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Invalid token. Please login first.",
		})
		return
	}

	// Get user's first tunnel
	tunnels, err := s.tunnelRepo.GetByUserID(context.Background(), token.UserID)
	if err != nil || len(tunnels) == 0 {
		s.logger.Error("No tunnels found for user: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "No tunnels found. Please create a tunnel first.",
		})
		return
	}

	tunnel := tunnels[0] // Use the first tunnel

	s.logger.Info("User %d connected with token %s", tunnel.UserID, tunnel.Token)

	// Get client IP from connection
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		s.logger.Error("Failed to get client IP: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to get client IP",
		})
		return
	}

	// Update client IP using tunnel service
	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP: %v", err)
		encoder.Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to update client IP",
		})
		return
	}

	// Send success response with domain and port
	if err := encoder.Encode(TunnelHandshakeResponse{
		Status:     "success",
		Message:    "Connected successfully",
		Domain:     tunnel.Domain,
		TargetPort: tunnel.TargetPort,
	}); err != nil {
		s.logger.Error("Failed to send response: %v", err)
		return
	}

	// Create connection object
	connection := s.connections.AddConnection(tunnel.Domain, conn, tunnel.TargetPort)
	connection.reader = decoder
	connection.writer = encoder
	defer s.connections.RemoveConnection(tunnel.Domain)

	// Create channels for different message types
	dataChan := make(chan *TunnelMessage, 100)    // Buffer for data messages
	controlChan := make(chan *TunnelMessage, 100) // Buffer for control messages
	errChan := make(chan error, 2)                // Error channel
	stopChan := make(chan struct{})               // Stop channel
	defer close(stopChan)

	// Start message reader goroutine
	go func() {
		defer close(dataChan)
		defer close(controlChan)

		for {
			select {
			case <-stopChan:
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
						s.logger.Warn("Control channel buffer full, dropping message")
					}
				case MessageTypeData:
					select {
					case dataChan <- &msg:
					default:
						s.logger.Warn("Data channel buffer full, dropping message")
					}
				default:
					s.logger.Error("Unknown message type: %s", msg.Type)
				}
			}
		}
	}()

	// Start ping handler goroutine
	go func() {
		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-pingTicker.C:
				// Generate unique message ID for correlation
				msgID := fmt.Sprintf("ping-%d", time.Now().UnixNano())

				// Send ping
				pingPayload, _ := json.Marshal(PingMessage{
					Timestamp: time.Now().UnixNano(),
				})
				pingMsg := TunnelMessage{
					Type:    MessageTypePing,
					ID:      msgID,
					Payload: pingPayload,
				}

				// Set write deadline for ping
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := encoder.Encode(pingMsg); err != nil {
					errChan <- fmt.Errorf("error sending ping: %w", err)
					return
				}
				conn.SetWriteDeadline(time.Time{})

				// Wait for matching pong
				pongTimer := time.NewTimer(5 * time.Second)
				defer pongTimer.Stop()
				pongReceived := false

				for !pongReceived {
					select {
					case msg := <-controlChan:
						if msg.Type == MessageTypePong && msg.ID == msgID {
							var pongResp PongMessage
							if err := json.Unmarshal(msg.Payload, &pongResp); err != nil {
								s.logger.Error("Error unmarshaling pong: %v", err)
								continue
							}
							rtt := time.Duration(pongResp.RTT) * time.Nanosecond
							s.logger.Debug("Ping successful, RTT: %v", rtt)
							connection.lastPing = time.Now()
							pongReceived = true
						}
					case <-pongTimer.C:
						errChan <- fmt.Errorf("ping timeout")
						return
					case <-stopChan:
						return
					}
				}
			}
		}
	}()

	// Start data handler goroutine
	go func() {
		for {
			select {
			case <-stopChan:
				return
			case msg := <-dataChan:
				var dataPayload DataMessage
				if err := json.Unmarshal(msg.Payload, &dataPayload); err != nil {
					s.logger.Error("Error unmarshaling data message: %v", err)
					continue
				}

				// Process data message
				// This is where you would handle the actual tunnel data
				// For now, we'll just log it
				s.logger.Debug("Received data message of length: %d", len(dataPayload.Data))
			}
		}
	}()

	// Wait for any error
	if err := <-errChan; err != nil {
		s.logger.Error("Connection error: %v", err)
	}
}

// GetConnection returns the tunnel connection for a domain
func (s *TunnelServer) GetConnection(domain string) *TunnelConnection {
	return s.connections.GetConnection(domain)
}

// IsTunnelDomain returns true if the domain has an active tunnel
func (s *TunnelServer) IsTunnelDomain(domain string) bool {
	return s.connections.HasDomain(domain)
}

// ProxyConnection handles proxying an HTTP connection to the appropriate tunnel
func (s *TunnelServer) ProxyConnection(domain string, conn net.Conn) {
	defer conn.Close()

	tunnelConn := s.connections.GetConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No tunnel connection found for domain: %s", domain)
		return
	}

	s.logger.Info("[PROXY DEBUG] Starting proxy connection for domain: %s", domain)

	// Set TCP keep-alive to prevent connection from being closed prematurely
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetReadBuffer(32 * 1024)  // 32KB read buffer
		tcpConn.SetWriteBuffer(32 * 1024) // 32KB write buffer
	}

	// Create buffered reader for the client connection
	clientReader := bufio.NewReaderSize(conn, 32*1024)

	// Read the entire request into a buffer
	var requestData bytes.Buffer
	if _, err := io.Copy(&requestData, clientReader); err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading request: %v", err)
		return
	}

	// Send the request data through the tunnel
	dataMsg := DataMessage{
		Data: requestData.Bytes(),
	}
	payload, _ := json.Marshal(dataMsg)
	msg := TunnelMessage{
		Type:    MessageTypeData,
		Payload: payload,
	}
	if err := tunnelConn.writer.Encode(msg); err != nil {
		s.logger.Error("[PROXY DEBUG] Error sending request data: %v", err)
		return
	}

	// Wait for response data
	var responseMsg TunnelMessage
	if err := tunnelConn.reader.Decode(&responseMsg); err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading response data: %v", err)
		return
	}

	if responseMsg.Type != MessageTypeData {
		s.logger.Error("[PROXY DEBUG] Unexpected message type in response: %s", responseMsg.Type)
		return
	}

	var dataMsg DataMessage
	if err := json.Unmarshal(responseMsg.Payload, &dataMsg); err != nil {
		s.logger.Error("[PROXY DEBUG] Error unmarshaling response data: %v", err)
		return
	}

	// Write response back to client
	if _, err := conn.Write(dataMsg.Data); err != nil {
		s.logger.Error("[PROXY DEBUG] Error writing response to client: %v", err)
		return
	}

	s.logger.Info("[PROXY DEBUG] Request/response cycle completed successfully")
}