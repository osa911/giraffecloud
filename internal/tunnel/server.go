package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/service"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
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
	tokenRepo     repository.TokenRepository
}

// Connection represents an active tunnel connection
type Connection struct {
	conn         net.Conn
	tunnel       *ent.Tunnel
	stopChan     chan struct{}
	yamuxSession *yamux.Session
}

func loadClientCAs(caPath string) *x509.CertPool {
	caCert, err := ioutil.ReadFile(caPath)
	if err != nil {
		panic(fmt.Sprintf("failed to read CA cert: %v", err))
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		panic("failed to append CA cert to pool")
	}
	return pool
}

// NewServer creates a new tunnel server instance
func NewServer(tunnelService service.TunnelService, tokenRepo repository.TokenRepository) *TunnelServer {
	return &TunnelServer{
		tunnelService: tunnelService,
		logger:        logging.GetGlobalLogger(),
		stopChan:      make(chan struct{}),
		connections:   make(map[string]*Connection),
		tlsConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair("/app/certs/tunnel.crt", "/app/certs/tunnel.key")
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate: %w", err)
				}
				return &cert, nil
			},
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  loadClientCAs("/app/certs/ca.crt"),
		},
		tokenRepo: tokenRepo,
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

	// --- Enhanced Logging for Token Lookup ---
	tokenMasked := "<empty>"
	if len(req.Token) > 6 {
		tokenMasked = "***" + req.Token[len(req.Token)-6:]
	} else if req.Token != "" {
		tokenMasked = "***" + req.Token
	}
	s.logger.Info("Token: %s", req.Token)
	s.logger.Info("Looking up user by token: %s from %s", tokenMasked, conn.RemoteAddr().String())

	// 1. Get user by token
	tokenRecord, err := s.tokenRepo.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to get user by token: %s from %s: %v", tokenMasked, conn.RemoteAddr().String(), err)
		if req.Token == "" {
			s.logger.Warn("Handshake failed: empty token from %s", conn.RemoteAddr().String())
		}
		resp := handshakeResponse{
			Status:  "error",
			Message: "Invalid token",
		}
		s.sendHandshakeResponse(conn, resp)
		return
	}
	s.logger.Info("User found: ID=%d for token: %s from %s", tokenRecord.UserID, tokenMasked, conn.RemoteAddr().String())

	// 2. Get all tunnels for this user
	tunnels, err := s.tunnelService.ListTunnels(context.Background(), tokenRecord.UserID)
	if err != nil || len(tunnels) == 0 {
		s.logger.Error("No tunnels found for userID=%d (token: %s) from %s", tokenRecord.UserID, tokenMasked, conn.RemoteAddr().String())
		resp := handshakeResponse{
			Status:  "error",
			Message: "No tunnels found for user",
		}
		s.sendHandshakeResponse(conn, resp)
		return
	}
	tunnel := tunnels[0]
	s.logger.Info("Using first tunnel: ID=%d, Domain=%s, TargetPort=%d for userID=%d from %s", tunnel.ID, tunnel.Domain, tunnel.TargetPort, tokenRecord.UserID, conn.RemoteAddr().String())

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
	s.logger.Info("RemoteAddr for client: %s", conn.RemoteAddr().String())
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	s.logger.Info("Client IP: %s", clientIP)
	if err != nil {
		s.logger.Error("Failed to get client IP from %s: %v", conn.RemoteAddr().String(), err)
		return
	}

	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP for tunnel ID %d: %v", tunnel.ID, err)
		return
	}
	s.logger.Info("Updated client IP to %s for tunnel ID %d", clientIP, tunnel.ID)

	// Fetch the updated tunnel from the DB for production robustness
	updatedTunnel, err := s.tunnelService.GetTunnel(context.Background(), tokenRecord.UserID, uint32(tunnel.ID))
	if err != nil {
		s.logger.Error("Failed to fetch updated tunnel for tunnel ID %d: %v", tunnel.ID, err)
		return
	}

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

	// Create yamux session
	session, err := yamux.Server(conn, nil)
	if err != nil {
		s.logger.Error("Failed to create yamux session: %v", err)
		return
	}
	connection := &Connection{
		conn:         conn,
		tunnel:       updatedTunnel,
		stopChan:     make(chan struct{}),
		yamuxSession: session,
	}

	// Store connection
	s.mu.Lock()
	s.connections[req.Token] = connection
	s.mu.Unlock()
	s.logger.Info("Stored connection for tunnel ID %d from %s", tunnel.ID, conn.RemoteAddr().String())

	// Block until the connection is closed
	<-connection.stopChan
	// Now cleanup will run
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

// ProxyHTTPConnection proxies an incoming HTTP connection (from Caddy) to the correct tunnel client via yamux
func (s *TunnelServer) ProxyHTTPConnection(domain string, httpConn net.Conn) {
	s.mu.RLock()
	var tunnelConn *Connection
	for _, c := range s.connections {
		if c.tunnel.Domain == domain {
			tunnelConn = c
			break
		}
	}
	s.mu.RUnlock()
	if tunnelConn == nil {
		s.logger.Error("No tunnel connection for domain %s", domain)
		httpConn.Close()
		return
	}
	s.proxyConnection(tunnelConn, httpConn)
}

// proxyConnection handles proxying data between the tunnel and local service
func (s *TunnelServer) proxyConnection(tunnelConn *Connection, httpConn net.Conn) {
	session := tunnelConn.yamuxSession
	if session == nil {
		s.logger.Error("No yamux session for tunnel ID %d", tunnelConn.tunnel.ID)
		httpConn.Close()
		return
	}
	stream, err := session.Open()
	if err != nil {
		s.logger.Error("Failed to open yamux stream for tunnel ID %d: %v", tunnelConn.tunnel.ID, err)
		httpConn.Close()
		return
	}
	defer stream.Close()
	s.logger.Info("Opened yamux stream to client for tunnel ID %d", tunnelConn.tunnel.ID)

	// Write JSON header to stream
	header := map[string]interface{}{
		"domain":    tunnelConn.tunnel.Domain,
		"local_port": tunnelConn.tunnel.TargetPort,
		"protocol":  "tcp",
	}
	headerBytes, _ := json.Marshal(header)
	headerBytes = append(headerBytes, '\n')
	stream.Write(headerBytes)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.logger.Info("Starting io.Copy from httpConn to stream (Caddy->yamux)")
		n, err := io.Copy(stream, httpConn)
		s.logger.Info("Copied %d bytes from httpConn to stream (Caddy->yamux), err=%v", n, err)
	}()
	go func() {
		defer wg.Done()
		s.logger.Info("Starting io.Copy from stream to httpConn (yamux->Caddy)")
		n, err := io.Copy(httpConn, stream)
		s.logger.Info("Copied %d bytes from stream to httpConn (yamux->Caddy), err=%v", n, err)
	}()
	wg.Wait()
	s.logger.Info("Proxy connection closed for tunnel ID %d", tunnelConn.tunnel.ID)
}

// IsTunnelDomain returns true if the given domain is an active tunnel
func (s *TunnelServer) IsTunnelDomain(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.connections {
		if c.tunnel.Domain == domain {
			return true
		}
	}
	return false
}