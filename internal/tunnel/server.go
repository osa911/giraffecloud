package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/service"
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
	listener      net.Listener
	tunnelService service.TunnelService
	logger        *logging.Logger
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	connections   map[string]*Connection // token -> connection
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
	}
}

// Start starts the tunnel server
func (s *TunnelServer) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create TCP listener (TLS is handled by Caddy)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

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

	// Read handshake message
	msg, err := readHandshakeMessage(conn)
	if err != nil {
		s.logger.Error("Failed to read handshake message: %v", err)
		return
	}

	// Parse handshake request
	var req handshakeRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		s.logger.Error("Failed to unmarshal handshake request: %v", err)
		return
	}

	// Validate token and get tunnel config
	tunnel, err := s.tunnelService.GetByToken(context.Background(), req.Token)
	if err != nil {
		s.logger.Error("Failed to get tunnel by token: %v", err)
		resp := handshakeResponse{
			Status:  "error",
			Message: "Invalid token",
		}
		s.sendHandshakeResponse(conn, resp)
		return
	}

	// Update client IP
	clientIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		s.logger.Error("Failed to get client IP: %v", err)
		return
	}

	if err := s.tunnelService.UpdateClientIP(context.Background(), uint32(tunnel.ID), clientIP); err != nil {
		s.logger.Error("Failed to update client IP: %v", err)
		return
	}

	// Send success response
	resp := handshakeResponse{
		Status:  "success",
		Message: "Connected successfully",
	}
	if err := s.sendHandshakeResponse(conn, resp); err != nil {
		s.logger.Error("Failed to send handshake response: %v", err)
		return
	}

	// Create connection object
	connection := &Connection{
		conn:     conn,
		tunnel:   tunnel,
		stopChan: make(chan struct{}),
	}

	// Store connection
	s.mu.Lock()
	s.connections[tunnel.Token] = connection
	s.mu.Unlock()

	// Start proxying
	s.proxyConnection(connection)

	// Clean up
	s.mu.Lock()
	delete(s.connections, tunnel.Token)
	s.mu.Unlock()
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
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		s.logger.Error("Failed to connect to target service: %v", err)
		return
	}
	defer targetConn.Close()

	// Start bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy from client to target
	go func() {
		defer wg.Done()
		if _, err := io.Copy(targetConn, conn.conn); err != nil {
			s.logger.Error("Error copying from client to target: %v", err)
		}
	}()

	// Copy from target to client
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn.conn, targetConn); err != nil {
			s.logger.Error("Error copying from target to client: %v", err)
		}
	}()

	// Wait for both copies to finish
	wg.Wait()
}