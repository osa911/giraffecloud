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
	"net/http"
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
	streamConfig  *StreamingConfig // Streaming configuration
}

// NewServer creates a new tunnel server instance
func NewServer(tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository, tunnelService interfaces.TunnelService) *TunnelServer {
	return &TunnelServer{
		logger:       logging.GetGlobalLogger(),
		connections:  NewConnectionManager(),
		streamConfig: DefaultStreamingConfig(), // Use default streaming config
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



// UpdateStreamingConfig updates the streaming configuration
func (s *TunnelServer) UpdateStreamingConfig(config *StreamingConfig) {
	s.streamConfig = config
	s.logger.Info("Updated streaming configuration: MediaOptimization=%v, PoolSize=%d, MediaBufferSize=%d",
		config.EnableMediaOptimization, config.PoolSize, config.MediaBufferSize)
}

// GetStreamingConfig returns the current streaming configuration
func (s *TunnelServer) GetStreamingConfig() *StreamingConfig {
	return s.streamConfig
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

	// Determine connection type based on request
	connType := ConnectionTypeHTTP
	if req.ConnectionType == "websocket" {
		connType = ConnectionTypeWebSocket
	}

	// Send success response with domain and port
	if err := encoder.Encode(TunnelHandshakeResponse{
		Status:     "success",
		Message:    "Connected successfully",
		Domain:     tunnel.Domain,
		TargetPort: tunnel.TargetPort,
		ConnectionType: string(connType),
	}); err != nil {
		s.logger.Error("Failed to send response: %v", err)
		return
	}

	// Create connection object and add to manager with type
	s.connections.AddConnection(tunnel.Domain, conn, tunnel.TargetPort, connType)
	defer s.connections.RemoveConnection(tunnel.Domain, connType)

	s.logger.Info("Tunnel connection established for domain: %s (type: %s)", tunnel.Domain, connType)

	// Keep the connection alive without interfering with HTTP traffic
	// The connection will be closed when the client disconnects or an error occurs
	// ProxyConnection will handle all HTTP communication
	select {}
}

// GetConnection returns the tunnel connection for a domain (backward compatibility for HTTP)
func (s *TunnelServer) GetConnection(domain string) *TunnelConnection {
	return s.connections.GetHTTPConnection(domain)
}

// IsTunnelDomain returns true if the domain has an active tunnel
func (s *TunnelServer) IsTunnelDomain(domain string) bool {
	return s.connections.HasDomain(domain)
}

// HasWebSocketConnection returns true if the domain has an active WebSocket tunnel
func (s *TunnelServer) HasWebSocketConnection(domain string) bool {
	return s.connections.HasWebSocketConnection(domain)
}

// ProxyConnection handles proxying an HTTP connection to the appropriate tunnel
func (s *TunnelServer) ProxyConnection(domain string, conn net.Conn, requestData []byte, requestBody io.Reader) {
	defer conn.Close()

	tunnelConn := s.connections.GetHTTPConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No HTTP tunnel connection found for domain: %s", domain)
		s.writeHTTPError(conn, 502, "Bad Gateway - HTTP tunnel not connected")
		return
	}

	// Check if this is a media/video request that should bypass serialization
	isMediaRequest := s.isMediaRequest(requestData)

	if isMediaRequest {
		// For media requests, use a separate connection to avoid blocking other requests
		s.logger.Info("[PROXY DEBUG] Detected media request, using optimized streaming for domain: %s", domain)
		s.proxyMediaRequest(domain, conn, requestData, requestBody)
		return
	}

	// Lock the tunnel connection to prevent concurrent access for regular requests
	// This ensures only one HTTP request/response cycle happens at a time for non-media
	tunnelConn.Lock()
	defer tunnelConn.Unlock()

	s.logger.Info("[PROXY DEBUG] Starting HTTP proxy for domain: %s", domain)

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.conn.Write(requestData); err != nil {
		s.logger.Error("[PROXY DEBUG] Error writing request headers to tunnel: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[PROXY DEBUG] Sent HTTP request headers to tunnel")

	// Copy request body if present
	if requestBody != nil {
		written, err := io.Copy(tunnelConn.conn, requestBody)
		if err != nil {
			s.logger.Error("[PROXY DEBUG] Error writing request body to tunnel: %v", err)
			s.writeHTTPError(conn, 502, "Bad Gateway")
			return
		}
		s.logger.Info("[PROXY DEBUG] Sent %d bytes of request body to tunnel", written)
	} else {
		s.logger.Info("[PROXY DEBUG] Sent 0 bytes of request body to tunnel")
	}

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.conn)

	// Parse the response
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[PROXY DEBUG] Error reading response from tunnel: %v", err)
		s.writeHTTPError(conn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[PROXY DEBUG] Received HTTP response: %s", response.Status)

	// Write the response back to the client
	clientWriter := bufio.NewWriter(conn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Error("[PROXY DEBUG] Error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Error("[PROXY DEBUG] Error flushing response: %v", err)
		return
	}

	s.logger.Info("[PROXY DEBUG] HTTP proxy completed successfully")
}

// isMediaRequest checks if the request is for media content that should be streamed
func (s *TunnelServer) isMediaRequest(requestData []byte) bool {
	if !s.streamConfig.EnableMediaOptimization {
		return false
	}

	requestStr := string(requestData)

	// Check for media file extensions from config
	for _, ext := range s.streamConfig.MediaExtensions {
		if strings.Contains(requestStr, ext) {
			return true
		}
	}

	// Check for Range requests (common for video streaming)
	if strings.Contains(requestStr, "Range:") {
		return true
	}

	// Check for media paths from config
	for _, path := range s.streamConfig.MediaPaths {
		if strings.Contains(requestStr, path) {
			return true
		}
	}

	return false
}



// proxyMediaRequest handles media requests with optimized streaming through the tunnel
func (s *TunnelServer) proxyMediaRequest(domain string, clientConn net.Conn, requestData []byte, requestBody io.Reader) {
	// Get the tunnel connection
	tunnelConn := s.connections.GetHTTPConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No tunnel connection found for media request to domain: %s", domain)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - Tunnel not connected")
		return
	}

	s.logger.Info("[MEDIA PROXY] Using tunnel connection for media streaming")

	// For media requests, we don't lock the tunnel to allow concurrent media streaming
	// This allows multiple video segments to be requested simultaneously

	// Write the HTTP request headers to the tunnel connection
	if _, err := tunnelConn.conn.Write(requestData); err != nil {
		s.logger.Error("[MEDIA PROXY] Error writing request headers to tunnel: %v", err)
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[MEDIA PROXY] Sent HTTP request headers to tunnel")

	// Copy request body if present
	if requestBody != nil {
		written, err := io.Copy(tunnelConn.conn, requestBody)
		if err != nil {
			s.logger.Error("[MEDIA PROXY] Error writing request body to tunnel: %v", err)
			s.writeHTTPError(clientConn, 502, "Bad Gateway")
			return
		}
		s.logger.Info("[MEDIA PROXY] Sent %d bytes of request body to tunnel", written)
	} else {
		s.logger.Info("[MEDIA PROXY] Sent 0 bytes of request body to tunnel")
	}

	// Read the HTTP response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.conn)

	// Parse the response
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[MEDIA PROXY] Error reading response from tunnel: %v", err)
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[MEDIA PROXY] Received HTTP response: %s", response.Status)

	// Write the response back to the client with optimized streaming
	clientWriter := bufio.NewWriter(clientConn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Error("[MEDIA PROXY] Error writing response to client: %v", err)
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Error("[MEDIA PROXY] Error flushing response: %v", err)
		return
	}

	s.logger.Info("[MEDIA PROXY] Media streaming completed successfully")
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

// ProxyWebSocketConnection handles WebSocket upgrade and bidirectional forwarding
func (s *TunnelServer) ProxyWebSocketConnection(domain string, clientConn net.Conn, r *http.Request) {
	defer clientConn.Close()

	tunnelConn := s.connections.GetWebSocketConnection(domain)
	if tunnelConn == nil {
		s.logger.Error("No WebSocket tunnel connection found for domain: %s", domain)
		s.writeHTTPError(clientConn, 502, "Bad Gateway - WebSocket tunnel not connected")
		return
	}

	s.logger.Info("[WEBSOCKET DEBUG] Starting WebSocket proxy for domain: %s", domain)

	// Build the WebSocket upgrade request
	var requestData strings.Builder

	// Add request line
	requestData.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, r.URL.RequestURI()))

	// Add Host header first
	requestData.WriteString(fmt.Sprintf("Host: %s\r\n", r.Host))

	// Add all headers (WebSocket upgrade headers are critical)
	for key, values := range r.Header {
		if key != "Host" { // Skip Host as we already added it
			for _, value := range values {
				requestData.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
			}
		}
	}

	// Add empty line to separate headers from body
	requestData.WriteString("\r\n")

	// Get the request as bytes
	requestBytes := []byte(requestData.String())
	s.logger.Info("[WEBSOCKET DEBUG] Forwarding WebSocket upgrade request:\n%s", requestData.String())

	// Lock the tunnel connection only for the upgrade handshake
	tunnelConn.Lock()

	// Send the upgrade request to the tunnel
	if _, err := tunnelConn.conn.Write(requestBytes); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade request to tunnel: %v", err)
		tunnelConn.Unlock()
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[WEBSOCKET DEBUG] Sent WebSocket upgrade request to tunnel")

	// Read the upgrade response from the tunnel
	tunnelReader := bufio.NewReader(tunnelConn.conn)
	response, err := http.ReadResponse(tunnelReader, nil)
	if err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error reading upgrade response from tunnel: %v", err)
		tunnelConn.Unlock()
		s.writeHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.logger.Info("[WEBSOCKET DEBUG] Received upgrade response: %s", response.Status)

	// Write the upgrade response back to the client
	clientWriter := bufio.NewWriter(clientConn)
	if err := response.Write(clientWriter); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error writing upgrade response to client: %v", err)
		tunnelConn.Unlock()
		return
	}

	if err := clientWriter.Flush(); err != nil {
		s.logger.Error("[WEBSOCKET DEBUG] Error flushing upgrade response: %v", err)
		tunnelConn.Unlock()
		return
	}

	// Check if the upgrade was successful (101 Switching Protocols)
	if response.StatusCode != 101 {
		s.logger.Error("[WEBSOCKET DEBUG] WebSocket upgrade failed with status: %d", response.StatusCode)
		tunnelConn.Unlock()
		return
	}

	s.logger.Info("[WEBSOCKET DEBUG] WebSocket upgrade successful, starting bidirectional forwarding")

	// Unlock the tunnel connection after successful upgrade
	// We don't need serialization for WebSocket data forwarding
	tunnelConn.Unlock()

	// Start bidirectional copying
	errChan := make(chan error, 2)

	// Copy from client to tunnel
	go func() {
		_, err := io.Copy(tunnelConn.conn, clientConn)
		errChan <- err
	}()

	// Copy from tunnel to client
	go func() {
		_, err := io.Copy(clientConn, tunnelConn.conn)
		errChan <- err
	}()

	// Wait for either direction to close or error
	err = <-errChan
	if err != nil {
		s.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed: %v", err)
	} else {
		s.logger.Info("[WEBSOCKET DEBUG] WebSocket connection closed normally")
	}

	s.logger.Info("[WEBSOCKET DEBUG] WebSocket proxy completed")
}