package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/service"
	"io"
	"net"
	"os"
	"sync"
	"time"
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
	listener      net.Listener
	tunnelService service.TunnelService
	logger        *logging.Logger
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	connections   map[string]*Connection // token -> connection
	tlsConfig     *tls.Config
}

// Connection represents an active tunnel connection
type Connection struct {
	conn     net.Conn
	tunnel   *ent.Tunnel
	stopChan chan struct{}
}

// NewServer creates a new tunnel server instance
func NewServer(tunnelService service.TunnelService) *TunnelServer {
	return &TunnelServer{
		tunnelService: tunnelService,
		logger:        logging.GetGlobalLogger(),
		stopChan:      make(chan struct{}),
		connections:   make(map[string]*Connection),
		tlsConfig:     &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs: func() *x509.CertPool {
				pool := x509.NewCertPool()
				if caCert, err := os.ReadFile("/app/certs/ca.crt"); err == nil {
					pool.AppendCertsFromPEM(caCert)
				}
				return pool
			}(),
		},
	}
}

// Start starts the tunnel server
func (s *TunnelServer) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create TCP listener
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	// Wrap with TLS
	s.listener = tls.NewListener(tcpListener, s.tlsConfig)

	s.wg.Add(1)
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

	// Signal stop
	close(s.stopChan)

	// Close listener
	if err := s.listener.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}

	// Close all active connections
	for _, conn := range s.connections {
		close(conn.stopChan)
		conn.conn.Close()
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.listener = nil
	s.stopChan = make(chan struct{})
	s.connections = make(map[string]*Connection)

	return nil
}

// acceptConnections accepts incoming connections
func (s *TunnelServer) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stopChan:
					return
				default:
					s.logger.Error("Failed to accept connection: %v", err)
					continue
				}
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles a new tunnel connection
func (s *TunnelServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	s.logger.Info("New tunnel connection from %s", conn.RemoteAddr().String())

	// Set read deadline for handshake
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		s.logger.Error("Failed to set read deadline for %s: %v", conn.RemoteAddr().String(), err)
		return
	}

	// Read handshake message
	msg, err := readHandshakeMessage(conn)
	if err != nil {
		s.logger.Error("Failed to read handshake message from %s: %v", conn.RemoteAddr().String(), err)
		return
	}
	s.logger.Info("Received handshake message from %s", conn.RemoteAddr().String())

	// Clear read deadline after handshake
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		s.logger.Error("Failed to clear read deadline for %s: %v", conn.RemoteAddr().String(), err)
		return
	}

	// Parse handshake request
	var req handshakeRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		s.logger.Error("Failed to unmarshal handshake request from %s: %v", conn.RemoteAddr().String(), err)
		return
	}
	s.logger.Info("Parsed handshake request from %s", conn.RemoteAddr().String())

	// Validate token and get tunnel config
	tunnel, err := s.tunnelService.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to get tunnel by token from %s: %v", conn.RemoteAddr().String(), err)
		resp := handshakeResponse{
			Status:  "error",
			Message: "Invalid token",
		}
		s.sendHandshakeResponse(conn, resp)
		return
	}
	s.logger.Info("Validated token for tunnel ID %d from %s", tunnel.ID, conn.RemoteAddr().String())

	// Set up cleanup handler
	defer func() {
		// Ensure we clean up if the connection handler exits for any reason
		s.mu.Lock()
		if conn, exists := s.connections[req.Token]; exists {
			s.logger.Info("Cleaning up connection for tunnel ID %d from %s", tunnel.ID, conn.conn.RemoteAddr().String())
			close(conn.stopChan)
			delete(s.connections, req.Token)
			// Clear client IP and remove Caddy route
			if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), ""); err != nil {
				s.logger.Error("Failed to clear client IP on disconnect for tunnel ID %d: %v", tunnel.ID, err)
			}
		}
		s.mu.Unlock()
	}()

	// Update client IP and configure Caddy route
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		s.logger.Error("Failed to get client IP from %s: %v", conn.RemoteAddr().String(), err)
		return
	}

	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP for tunnel ID %d: %v", tunnel.ID, err)
		return
	}
	s.logger.Info("Updated client IP to %s for tunnel ID %d", clientIP, tunnel.ID)

	// Send success response
	resp := handshakeResponse{
		Status:  "success",
		Message: "Connected successfully",
	}
	if err := s.sendHandshakeResponse(conn, resp); err != nil {
		s.logger.Error("Failed to send handshake response to %s: %v", conn.RemoteAddr().String(), err)
		return
	}
	s.logger.Info("Sent success response to %s for tunnel ID %d", conn.RemoteAddr().String(), tunnel.ID)

	// Create connection object
	connection := &Connection{
		conn:     conn,
		tunnel:   tunnel,
		stopChan: make(chan struct{}),
	}

	// Store connection
	s.mu.Lock()
	s.connections[req.Token] = connection
	s.mu.Unlock()
	s.logger.Info("Stored connection for tunnel ID %d from %s", tunnel.ID, conn.RemoteAddr().String())

	// Start proxying
	s.proxyConnection(connection)
}

// sendHandshakeResponse sends a handshake response
func (s *TunnelServer) sendHandshakeResponse(conn net.Conn, resp handshakeResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	msg := &handshakeMessage{
		Type:    handshakeMsgTypeResponse,
		Payload: data,
	}

	return writeHandshakeMessage(conn, msg)
}

// proxyConnection handles proxying data between the tunnel and local service
func (s *TunnelServer) proxyConnection(conn *Connection) {
	// Create connection to target service
	target := fmt.Sprintf("%s:%d", conn.tunnel.ClientIP, conn.tunnel.TargetPort)
	s.logger.Info("Attempting to connect to target service at %s for tunnel ID %d", target, conn.tunnel.ID)

	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		s.logger.Error("Failed to connect to target service at %s for tunnel ID %d: %v", target, conn.tunnel.ID, err)
		return
	}
	defer targetConn.Close()
	s.logger.Info("Connected to target service at %s for tunnel ID %d", target, conn.tunnel.ID)

	// Start bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy from client to target
	go func() {
		defer wg.Done()
		if _, err := io.Copy(targetConn, conn.conn); err != nil {
			s.logger.Error("Error copying from client to target for tunnel ID %d: %v", conn.tunnel.ID, err)
		}
		s.logger.Info("Client to target copy finished for tunnel ID %d", conn.tunnel.ID)
	}()

	// Copy from target to client
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn.conn, targetConn); err != nil {
			s.logger.Error("Error copying from target to client for tunnel ID %d: %v", conn.tunnel.ID, err)
		}
		s.logger.Info("Target to client copy finished for tunnel ID %d", conn.tunnel.ID)
	}()

	// Wait for both copies to finish
	wg.Wait()
	s.logger.Info("Proxy connection closed for tunnel ID %d", conn.tunnel.ID)
}