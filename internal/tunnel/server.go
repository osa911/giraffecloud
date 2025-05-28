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
	bufferPool    *BufferPool
	healthChecker *HealthChecker
}

// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository, tunnelService interfaces.TunnelService) *TunnelServer {
	return &TunnelServer{
		logger:        logging.GetGlobalLogger(),
		connections:   NewConnectionManager(),
		bufferPool:    NewBufferPool(),
		healthChecker: NewHealthChecker(DefaultHealthCheckConfig()),
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true, // Simplified for development
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

	// Create connection object and add to manager
	connection := s.connections.AddConnection(tunnel.Domain, conn, tunnel.TargetPort)
	defer s.connections.RemoveConnection(tunnel.Domain)

	s.logger.Info("Tunnel connection established for domain: %s", tunnel.Domain)

	// Simple connection keeper - just read simple pings and stay alive
	buffer := make([]byte, 1024)
	for {
		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Try to read from connection
		_, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is normal, continue
				continue
			}
			// Connection closed or error
			s.logger.Info("Tunnel connection closed for domain: %s", tunnel.Domain)
			break
		}

		// Update last ping time
		connection.lastPing = time.Now()

		// Reset deadline
		conn.SetReadDeadline(time.Time{})
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
		s.writeHTTPError(conn, 502, "Bad Gateway - Tunnel not connected")
		return
	}

	// Check tunnel health before proceeding
	if tunnelConn.healthChecker.GetStatus() == StatusUnhealthy {
		s.logger.Error("Tunnel is unhealthy for domain: %s", domain)
		s.writeHTTPError(conn, 503, "Service Unavailable - Tunnel is unhealthy")
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

	// Get buffer from pool for request data
	requestBuffer := s.bufferPool.Get()
	defer s.bufferPool.Put(requestBuffer)

	// Create buffered reader for the client connection
	clientReader := bufio.NewReaderSize(conn, 32*1024)

	// Read the entire request into a buffer with timeout
	var requestData bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := io.CopyBuffer(&requestData, clientReader, requestBuffer)
		readDone <- err
	}()

	select {
	case err := <-readDone:
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error reading request: %v", err)
			s.writeHTTPError(conn, 502, "Bad Gateway - Error reading request")
			return
		}
	case <-time.After(60 * time.Second):
		s.logger.Error("[PROXY DEBUG] Timeout reading request")
		s.writeHTTPError(conn, 504, "Gateway Timeout - Request read timeout")
		return
	}

	// Update connection stats
	tunnelConn.stateManager.AddBytes(uint64(requestData.Len()), 0)
	tunnelConn.stateManager.IncrementRequests()

	// Generate unique message ID for correlation
	msgID := fmt.Sprintf("proxy-%d", time.Now().UnixNano())

	// Send the request data through the tunnel
	requestDataMsg := DataMessage{
		Data: requestData.Bytes(),
	}
	payload, _ := json.Marshal(requestDataMsg)
	msg := TunnelMessage{
		Type:    MessageTypeData,
		ID:      msgID,
		Payload: payload,
	}

	// Lock the writer mutex with timeout
	writerLock := make(chan struct{}, 1)
	go func() {
		tunnelConn.writerMu.Lock()
		writerLock <- struct{}{}
	}()

	select {
	case <-writerLock:
		// Got the lock, proceed
	case <-time.After(10 * time.Second):
		s.logger.Error("[PROXY DEBUG] Timeout acquiring writer lock")
		s.writeHTTPError(conn, 504, "Gateway Timeout - Internal lock timeout")
		return
	}

	// Ensure we unlock the writer mutex
	defer tunnelConn.writerMu.Unlock()

	// Set write deadline for the tunnel connection
	tunnelConn.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := tunnelConn.writer.Encode(msg); err != nil {
		s.logger.Error("[PROXY DEBUG] Error sending request data: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Error sending request to tunnel")
		return
	}
	tunnelConn.conn.SetWriteDeadline(time.Time{})

	// Get buffer from pool for response data
	responseBuffer := s.bufferPool.Get()
	defer s.bufferPool.Put(responseBuffer)

	// Wait for response data with timeout
	responseChan := make(chan *TunnelMessage, 1)
	errorChan := make(chan error, 1)

	go func() {
		tunnelConn.readerMu.Lock()
		defer tunnelConn.readerMu.Unlock()

		var responseMsg TunnelMessage
		if err := tunnelConn.reader.Decode(&responseMsg); err != nil {
			errorChan <- fmt.Errorf("error reading response data: %w", err)
			return
		}

		if responseMsg.Type != MessageTypeData {
			errorChan <- fmt.Errorf("unexpected message type in response: %s", responseMsg.Type)
			return
		}

		if responseMsg.ID != msgID {
			errorChan <- fmt.Errorf("response message ID mismatch: got %s, want %s", responseMsg.ID, msgID)
			return
		}

		responseChan <- &responseMsg
	}()

	// Wait for response with increased timeout
	select {
	case responseMsg := <-responseChan:
		var responseDataMsg DataMessage
		if err := json.Unmarshal(responseMsg.Payload, &responseDataMsg); err != nil {
			s.logger.Error("[PROXY DEBUG] Error unmarshaling response data: %v", err)
			s.writeHTTPError(conn, 502, "Bad Gateway - Error processing response")
			return
		}

		// Update connection stats
		tunnelConn.stateManager.AddBytes(0, uint64(len(responseDataMsg.Data)))

		// Write response back to client with timeout
		conn.SetWriteDeadline(time.Now().Add(60 * time.Second))
		if _, err := conn.Write(responseDataMsg.Data); err != nil {
			s.logger.Error("[PROXY DEBUG] Error writing response to client: %v", err)
			return
		}

		s.logger.Info("[PROXY DEBUG] Request/response cycle completed successfully")

	case err := <-errorChan:
		s.logger.Error("[PROXY DEBUG] Error in response handling: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway - Error receiving response from tunnel")
		return

	case <-time.After(120 * time.Second):
		s.logger.Error("[PROXY DEBUG] Timeout waiting for response")
		s.writeHTTPError(conn, 504, "Gateway Timeout - Response timeout")
		return
	}
}

// writeHTTPError writes a proper HTTP error response
func (s *TunnelServer) writeHTTPError(conn net.Conn, code int, message string) {
	statusText := "Bad Gateway"
	if code == 504 {
		statusText = "Gateway Timeout"
	}

	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Type: text/plain\r\n"+
		"Connection: close\r\n"+
		"\r\n"+
		"%s", code, statusText, message)

	conn.Write([]byte(response))
}