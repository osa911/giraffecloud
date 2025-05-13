package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"io"
	"net"
	"sync"
)

/**
	FYI: We use this server to handle the handshake and proxy the connection to the local service

TODO:
- Add rate limiting
- Add logging
- Add metrics
*/

// TunnelServer represents the tunnel server
type TunnelServer struct {
	listener    net.Listener
	logger      *logging.Logger
	mu          sync.RWMutex
	connections map[string]*TunnelConnection // domain -> connection
	tlsConfig   *tls.Config
	tokenRepo   repository.TokenRepository
}

// Connection represents an active tunnel connection
type Connection struct {
	conn     net.Conn
	domain   string
	stopChan chan struct{}
}

// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository) *TunnelServer {
	return &TunnelServer{
		logger:      logging.GetGlobalLogger(),
		connections: make(map[string]*TunnelConnection),
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
		tokenRepo: tokenRepo,
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return nil
	}

	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	for _, conn := range s.connections {
		close(conn.stopChan)
		conn.conn.Close()
	}

	s.connections = make(map[string]*TunnelConnection)
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

	// Validate token
	tokenRecord, err := s.tokenRepo.GetByToken(context.Background(), req.Token)
	if err != nil {
		json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
			Status:  "error",
			Message: "Invalid token",
		})
		return
	}

	s.logger.Info("User %d connected with domain %s", tokenRecord.UserID, req.Domain)

	// Store connection
	connection := &TunnelConnection{
		conn:     conn,
		domain:   req.Domain,
		stopChan: make(chan struct{}),
	}

	s.mu.Lock()
	s.connections[req.Domain] = connection
	s.mu.Unlock()

	// Send success response
	if err := json.NewEncoder(conn).Encode(TunnelHandshakeResponse{
		Status:  "success",
		Message: "Connected successfully",
	}); err != nil {
		s.logger.Error("Failed to send response: %v", err)
		return
	}

	// Wait for connection to close
	<-connection.stopChan

	// Cleanup
	s.mu.Lock()
	delete(s.connections, req.Domain)
	s.mu.Unlock()
}

// ProxyHTTPConnection proxies an incoming HTTP connection to the tunnel client
func (s *TunnelServer) ProxyHTTPConnection(domain string, httpConn net.Conn) {
	s.mu.RLock()
	tunnelConn := s.connections[domain]
	s.mu.RUnlock()

	if tunnelConn == nil {
		s.logger.Error("No tunnel connection for domain %s", domain)
		httpConn.Close()
		return
	}

	// Copy data bidirectionally
	go func() {
		io.Copy(tunnelConn.conn, httpConn)
		httpConn.Close()
	}()

	go func() {
		io.Copy(httpConn, tunnelConn.conn)
		tunnelConn.conn.Close()
	}()
}

// IsTunnelDomain returns true if the domain has an active tunnel
func (s *TunnelServer) IsTunnelDomain(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.connections[domain]
	return exists
}