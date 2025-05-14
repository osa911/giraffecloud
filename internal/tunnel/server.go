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
	"strconv"
	"strings"
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

	if tcpConn, ok := tunnelConn.conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetReadBuffer(32 * 1024)  // 32KB read buffer
		tcpConn.SetWriteBuffer(32 * 1024) // 32KB write buffer
	}

	// Create buffered readers and writers
	clientReader := bufio.NewReaderSize(conn, 32*1024)
	clientWriter := bufio.NewWriterSize(conn, 32*1024)
	tunnelReader := bufio.NewReaderSize(tunnelConn.conn, 32*1024)
	tunnelWriter := bufio.NewWriterSize(tunnelConn.conn, 32*1024)

	// Create error channels for both directions
	errChan := make(chan error, 2)
	done := make(chan struct{})

	// Forward data from client to tunnel
	go func() {
		defer func() {
			s.logger.Info("[PROXY DEBUG] Client to tunnel goroutine completed")
			close(done)
		}()

		for {
			select {
			case <-done:
				return
			default:
				// Read request line
				requestLine, err := clientReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						s.logger.Error("[PROXY DEBUG] Error reading request line: %v", err)
						errChan <- err
					}
					return
				}
				s.logger.Info("[PROXY DEBUG] Request line: %s", strings.TrimSpace(requestLine))

				// Write request line
				if _, err := tunnelWriter.WriteString(requestLine); err != nil {
					s.logger.Error("[PROXY DEBUG] Error writing request line: %v", err)
					errChan <- err
					return
				}

				// Read and forward headers
				var contentLength int64
				for {
					line, err := clientReader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							s.logger.Error("[PROXY DEBUG] Error reading header: %v", err)
							errChan <- err
						}
						return
					}

					// Write header line
					if _, err := tunnelWriter.WriteString(line); err != nil {
						s.logger.Error("[PROXY DEBUG] Error writing header: %v", err)
						errChan <- err
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
				if err := tunnelWriter.Flush(); err != nil {
					s.logger.Error("[PROXY DEBUG] Error flushing headers: %v", err)
					errChan <- err
					return
				}

				// Forward request body if present
				if contentLength > 0 {
					s.logger.Info("[PROXY DEBUG] Forwarding request body of length: %d", contentLength)
					written, err := io.CopyN(tunnelWriter, clientReader, contentLength)
					if err != nil {
						s.logger.Error("[PROXY DEBUG] Error forwarding request body: %v", err)
						errChan <- err
						return
					}
					s.logger.Info("[PROXY DEBUG] Wrote %d bytes of request body", written)

					// Flush body
					if err := tunnelWriter.Flush(); err != nil {
						s.logger.Error("[PROXY DEBUG] Error flushing body: %v", err)
						errChan <- err
						return
					}
				}
			}
		}
	}()

	// Forward data from tunnel to client
	go func() {
		defer func() {
			s.logger.Info("[PROXY DEBUG] Tunnel to client goroutine completed")
			close(done)
		}()

		for {
			select {
			case <-done:
				return
			default:
				// Read response line
				responseLine, err := tunnelReader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						s.logger.Error("[PROXY DEBUG] Error reading response line: %v", err)
						errChan <- err
					}
					return
				}
				s.logger.Info("[PROXY DEBUG] Response line: %s", strings.TrimSpace(responseLine))

				// Write response line
				if _, err := clientWriter.WriteString(responseLine); err != nil {
					s.logger.Error("[PROXY DEBUG] Error writing response line: %v", err)
					errChan <- err
					return
				}

				// Read and forward headers
				var contentLength int64
				for {
					line, err := tunnelReader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							s.logger.Error("[PROXY DEBUG] Error reading response header: %v", err)
							errChan <- err
						}
						return
					}

					// Write header line
					if _, err := clientWriter.WriteString(line); err != nil {
						s.logger.Error("[PROXY DEBUG] Error writing response header: %v", err)
						errChan <- err
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
				if err := clientWriter.Flush(); err != nil {
					s.logger.Error("[PROXY DEBUG] Error flushing response headers: %v", err)
					errChan <- err
					return
				}

				// Forward response body if present
				if contentLength > 0 {
					s.logger.Info("[PROXY DEBUG] Forwarding response body of length: %d", contentLength)
					written, err := io.CopyN(clientWriter, tunnelReader, contentLength)
					if err != nil {
						s.logger.Error("[PROXY DEBUG] Error forwarding response body: %v", err)
						errChan <- err
						return
					}
					s.logger.Info("[PROXY DEBUG] Wrote %d bytes of response body", written)

					// Flush body
					if err := clientWriter.Flush(); err != nil {
						s.logger.Error("[PROXY DEBUG] Error flushing response body: %v", err)
						errChan <- err
						return
					}
				}
			}
		}
	}()

	// Wait for completion or error
	select {
	case err := <-errChan:
		s.logger.Error("[PROXY DEBUG] Proxy connection error: %v", err)
	case <-done:
		s.logger.Info("[PROXY DEBUG] Proxy connection completed normally")
	}
}