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
	"regexp"
	"strconv"
	"strings"
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

	// First, read the request line
	requestLine, err := clientReader.ReadString('\n')
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading request line: %v", err)
		conn.Close()
		return
	}
	s.logger.Info("[PROXY DEBUG] Request line: %s", strings.TrimSpace(requestLine))

	// Initialize request data with the request line
	requestData := []byte(requestLine)

	// Read headers until we hit an empty line
	for {
		line, err := clientReader.ReadString('\n')
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error reading header line: %v", err)
			conn.Close()
			return
		}
		requestData = append(requestData, []byte(line)...)

		// Check if we've reached the end of headers (empty line)
		if line == "\r\n" {
			break
		}
	}

	s.logger.Info("[PROXY DEBUG] Read request headers (%d bytes)", len(requestData))

	// Parse the request line to get the method
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) < 3 {
		s.logger.Error("[PROXY DEBUG] Invalid request line format")
		conn.Close()
		return
	}
	method := parts[0]

	// Check for Content-Length in headers
	contentLength := 0
	headers := string(requestData)
	if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
		contentLength, _ = strconv.Atoi(match[1])
	}

	// Read body for POST/PUT/PATCH methods or if Content-Length is present
	if (method == "POST" || method == "PUT" || method == "PATCH" || contentLength > 0) && !strings.Contains(headers, "Transfer-Encoding: chunked") {
		s.logger.Info("[PROXY DEBUG] Reading request body of length: %d", contentLength)
		body := make([]byte, contentLength)
		_, err := io.ReadFull(clientReader, body)
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error reading request body: %v", err)
			conn.Close()
			return
		}
		requestData = append(requestData, body...)
	} else if strings.Contains(headers, "Transfer-Encoding: chunked") {
		s.logger.Info("[PROXY DEBUG] Reading chunked request body")
		for {
			// Read chunk size line
			line, err := clientReader.ReadString('\n')
			if err != nil {
				s.logger.Error("[PROXY DEBUG] Error reading chunk size: %v", err)
				conn.Close()
				return
			}
			requestData = append(requestData, []byte(line)...)

			// Parse chunk size
			chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
			if err != nil || chunkSize == 0 {
				break
			}

			// Read chunk data
			chunk := make([]byte, chunkSize+2) // +2 for CRLF
			_, err = io.ReadFull(clientReader, chunk)
			if err != nil {
				s.logger.Error("[PROXY DEBUG] Error reading chunk data: %v", err)
				conn.Close()
				return
			}
			requestData = append(requestData, chunk...)
		}
		// Add final CRLF
		requestData = append(requestData, []byte("\r\n")...)
	}

	s.logger.Info("[PROXY DEBUG] Total request size: %d bytes", len(requestData))

	// Forward the request to the tunnel
	_, err = tunnelWriter.Write(requestData)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Failed to write request to tunnel: %v", err)
		conn.Close()
		return
	}
	err = tunnelWriter.Flush()
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Failed to flush request to tunnel: %v", err)
		conn.Close()
		return
	}
	s.logger.Info("[PROXY DEBUG] Forwarded request to tunnel")

	// Read the response status line
	statusLine, err := tunnelReader.ReadString('\n')
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading response status line: %v", err)
		conn.Close()
		return
	}
	s.logger.Info("[PROXY DEBUG] Response status: %s", strings.TrimSpace(statusLine))

	// Initialize response data with status line
	responseData := []byte(statusLine)

	// Read response headers
	for {
		line, err := tunnelReader.ReadString('\n')
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error reading response header: %v", err)
			conn.Close()
			return
		}
		responseData = append(responseData, []byte(line)...)

		// Check if we've reached the end of headers
		if line == "\r\n" {
			break
		}
	}

	// Check for Content-Length and Transfer-Encoding in response
	headers = string(responseData)
	contentLength = 0
	if match := regexp.MustCompile(`(?i)Content-Length: (\d+)`).FindStringSubmatch(headers); len(match) > 1 {
		contentLength, _ = strconv.Atoi(match[1])
	}

	// Read response body based on Content-Length or Transfer-Encoding
	if contentLength > 0 {
		s.logger.Info("[PROXY DEBUG] Reading response body of length: %d", contentLength)
		body := make([]byte, contentLength)
		_, err := io.ReadFull(tunnelReader, body)
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error reading response body: %v", err)
			conn.Close()
			return
		}
		responseData = append(responseData, body...)
	} else if strings.Contains(headers, "Transfer-Encoding: chunked") {
		s.logger.Info("[PROXY DEBUG] Reading chunked response body")
		for {
			// Read chunk size
			line, err := tunnelReader.ReadString('\n')
			if err != nil {
				s.logger.Error("[PROXY DEBUG] Error reading chunk size: %v", err)
				break
			}
			responseData = append(responseData, []byte(line)...)

			// Parse chunk size
			chunkSize, err := strconv.ParseInt(strings.TrimSpace(line), 16, 64)
			if err != nil || chunkSize == 0 {
				break
			}

			// Read chunk data
			chunk := make([]byte, chunkSize+2) // +2 for CRLF
			_, err = io.ReadFull(tunnelReader, chunk)
			if err != nil {
				s.logger.Error("[PROXY DEBUG] Error reading chunk data: %v", err)
				break
			}
			responseData = append(responseData, chunk...)
		}
		// Add final CRLF
		responseData = append(responseData, []byte("\r\n")...)
	}

	s.logger.Info("[PROXY DEBUG] Total response size: %d bytes", len(responseData))

	// Write the response back to the client
	_, err = clientWriter.Write(responseData)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Failed to write response to client: %v", err)
	} else {
		err = clientWriter.Flush()
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Failed to flush response to client: %v", err)
		} else {
			s.logger.Info("[PROXY DEBUG] Successfully wrote and flushed response to client")
		}
	}

	// Cleanup
	conn.Close()
	s.logger.Info("[PROXY DEBUG] Proxy connection completed for domain: %s", domain)
}