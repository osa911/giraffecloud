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
		_, err := io.Copy(tunnelWriter, clientReader)
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error copying client to tunnel: %v", err)
		}
		tunnelWriter.Flush()
		clientToTunnelErr <- err
	}()

	go func() {
		_, err := io.Copy(clientWriter, tunnelReader)
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error copying tunnel to client: %v", err)
		}
		clientWriter.Flush()
		tunnelToClientErr <- err
	}()

	// Wait for either direction to complete
	select {
	case err := <-clientToTunnelErr:
		if err != nil && err != io.EOF {
			s.logger.Error("[PROXY DEBUG] Client to tunnel error: %v", err)
		}
		tunnelWriter.Flush()
		// Wait for response data
		select {
		case err := <-tunnelToClientErr:
			if err != nil && err != io.EOF {
				s.logger.Error("[PROXY DEBUG] Tunnel to client error: %v", err)
			}
			clientWriter.Flush()
		case <-time.After(5 * time.Second):
			s.logger.Info("[PROXY DEBUG] Response wait timeout")
		}
	case err := <-tunnelToClientErr:
		if err != nil && err != io.EOF {
			s.logger.Error("[PROXY DEBUG] Tunnel to client error: %v", err)
		}
		clientWriter.Flush()
		// Wait for request completion
		select {
		case err := <-clientToTunnelErr:
			if err != nil && err != io.EOF {
				s.logger.Error("[PROXY DEBUG] Client to tunnel error: %v", err)
			}
			tunnelWriter.Flush()
		case <-time.After(5 * time.Second):
			s.logger.Info("[PROXY DEBUG] Request completion timeout")
		}
	case <-time.After(60 * time.Second):
		s.logger.Info("[PROXY DEBUG] Connection timeout")
	}

	close(done)
	s.logger.Info("[PROXY DEBUG] Proxy connection completed for domain: %s", domain)
}