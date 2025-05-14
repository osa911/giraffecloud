package tunnel

import (
	"bufio"
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

func (s *TunnelServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read handshake request
	var req TunnelHandshakeRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		s.logger.Error("Failed to decode handshake: %v", err)
		return
	}

	// Find user by API token
	token, err := s.tokenRepo.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to authenticate: %v", err)
		json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Invalid token. Please login first.",
		})
		return
	}

	// Get user's first tunnel
	tunnels, err := s.tunnelRepo.GetByUserID(context.Background(), token.UserID)
	if err != nil || len(tunnels) == 0 {
		s.logger.Error("No tunnels found for user: %v", err)
		json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
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
		json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to get client IP",
		})
		return
	}

	// Update client IP using tunnel service
	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP: %v", err)
		json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Failed to update client IP",
		})
		return
	}

	// Store connection
	connection := s.connections.AddConnection(tunnel.Domain, conn, tunnel.TargetPort)
	defer s.connections.RemoveConnection(tunnel.Domain)

	// Send success response with domain and port
	if err := json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
		Status:     "success",
		Message:    "Connected successfully",
		Domain:     tunnel.Domain,
		TargetPort: tunnel.TargetPort,
	}); err != nil {
		s.logger.Error("Failed to send response: %v", err)
		return
	}

	// Wait for connection to close
	<-connection.stopChan
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
	tunnelConn := s.connections.GetConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No tunnel connection found for domain: %s", domain)
		conn.Close()
		return
	}

	s.logger.Info("[PROXY DEBUG] Starting proxy connection for domain: %s", domain)

	// Set timeouts for the connections
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	tunnelConn.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	tunnelConn.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// Create buffered readers and writers with larger buffer sizes
	clientReader := bufio.NewReaderSize(conn, 32*1024) // 32KB buffer
	clientWriter := bufio.NewWriterSize(conn, 32*1024)
	tunnelReader := bufio.NewReaderSize(tunnelConn.conn, 32*1024)
	tunnelWriter := bufio.NewWriterSize(tunnelConn.conn, 32*1024)

	// Create error channels for both directions
	clientToTunnelErr := make(chan error, 1)
	tunnelToClientErr := make(chan error, 1)
	done := make(chan struct{})

	// Forward data in both directions concurrently
	go func() {
		n, err := io.Copy(tunnelWriter, clientReader)
		s.logger.Info("[PROXY DEBUG] Forwarded %d bytes from client to tunnel", n)
		if err := tunnelWriter.Flush(); err != nil {
			s.logger.Error("[PROXY DEBUG] Error flushing tunnel writer: %v", err)
		} else {
			s.logger.Info("[PROXY DEBUG] Successfully flushed tunnel writer")
		}
		clientToTunnelErr <- err
	}()

	go func() {
		n, err := io.Copy(clientWriter, tunnelReader)
		s.logger.Info("[PROXY DEBUG] Forwarded %d bytes from tunnel to client", n)
		if err := clientWriter.Flush(); err != nil {
			s.logger.Error("[PROXY DEBUG] Error flushing client writer: %v", err)
		} else {
			s.logger.Info("[PROXY DEBUG] Successfully flushed client writer")
		}
		tunnelToClientErr <- err
	}()

	// Set up a timeout for the entire proxy operation
	go func() {
		select {
		case <-time.After(60 * time.Second):
			s.logger.Info("[PROXY DEBUG] Connection timeout for domain: %s", domain)
			close(done)
		case <-done:
			return
		}
	}()

	// Wait for either direction to complete or error
	cleanup := func() {
		s.logger.Info("[PROXY DEBUG] Starting cleanup for domain: %s", domain)
		// Ensure all buffered data is written before closing
		if err := clientWriter.Flush(); err != nil {
			s.logger.Error("[PROXY DEBUG] Error flushing client writer during cleanup: %v", err)
		}
		if err := tunnelWriter.Flush(); err != nil {
			s.logger.Error("[PROXY DEBUG] Error flushing tunnel writer during cleanup: %v", err)
		}

		// Reset deadlines to prevent any lingering operations
		conn.SetReadDeadline(time.Time{})
		conn.SetWriteDeadline(time.Time{})
		tunnelConn.conn.SetReadDeadline(time.Time{})
		tunnelConn.conn.SetWriteDeadline(time.Time{})

		// Close connections
		if err := conn.Close(); err != nil {
			s.logger.Error("[PROXY DEBUG] Error closing client connection: %v", err)
		}

		close(done)
		s.logger.Info("[PROXY DEBUG] Cleanup completed for domain: %s", domain)
	}

	// Wait for data transfer to complete in both directions
	var clientToTunnelError, tunnelToClientError error
	select {
	case clientToTunnelError = <-clientToTunnelErr:
		s.logger.Info("[PROXY DEBUG] Client to tunnel transfer completed")
		// Wait a short time for response data
		select {
		case tunnelToClientError = <-tunnelToClientErr:
			s.logger.Info("[PROXY DEBUG] Tunnel to client transfer completed")
		case <-time.After(5 * time.Second):
			s.logger.Info("[PROXY DEBUG] Waiting for response timed out")
		}
	case tunnelToClientError = <-tunnelToClientErr:
		s.logger.Info("[PROXY DEBUG] Tunnel to client transfer completed first")
		// Wait for request to complete
		select {
		case clientToTunnelError = <-clientToTunnelErr:
			s.logger.Info("[PROXY DEBUG] Client to tunnel transfer completed")
		case <-time.After(5 * time.Second):
			s.logger.Info("[PROXY DEBUG] Waiting for request completion timed out")
		}
	case <-done:
		s.logger.Info("[PROXY DEBUG] Connection timed out")
	}

	// Log any non-EOF errors
	if clientToTunnelError != nil && clientToTunnelError != io.EOF {
		s.logger.Error("[PROXY DEBUG] Error forwarding client to tunnel: %v", clientToTunnelError)
	}
	if tunnelToClientError != nil && tunnelToClientError != io.EOF {
		s.logger.Error("[PROXY DEBUG] Error forwarding tunnel to client: %v", tunnelToClientError)
	}

	cleanup()
	s.logger.Info("[PROXY DEBUG] Proxy connection completed for domain: %s", domain)
}