package tunnel

import (
	"net"
)

// TunnelHandshakeRequest represents the initial handshake message
type TunnelHandshakeRequest struct {
	Token          string `json:"token"`
	ConnectionType string `json:"connection_type,omitempty"` // "http" or "websocket"
}

// TunnelHandshakeResponse represents the server's response to a handshake
type TunnelHandshakeResponse struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	Domain         string `json:"domain,omitempty"`
	TargetPort     int    `json:"target_port,omitempty"`
	ConnectionType string `json:"connection_type,omitempty"` // "http" or "websocket"
}

// TunnelConnection represents an active tunnel connection without serialization
// Concurrency is now handled at the pool level, allowing multiple concurrent HTTP requests
type TunnelConnection struct {
	conn       net.Conn    // The underlying network connection
	domain     string      // The domain this tunnel serves
	targetPort int         // The target port on the client side
	// Removed mutex - concurrency handled at pool level for better performance
}

// NewTunnelConnection creates a new tunnel connection
func NewTunnelConnection(domain string, conn net.Conn, targetPort int) *TunnelConnection {
	return &TunnelConnection{
		conn:       conn,
		domain:     domain,
		targetPort: targetPort,
	}
}

// Close closes the tunnel connection
func (tc *TunnelConnection) Close() error {
	if tc.conn != nil {
		return tc.conn.Close()
	}
	return nil
}

// GetConn returns the underlying network connection
func (tc *TunnelConnection) GetConn() net.Conn {
	return tc.conn
}

// GetDomain returns the domain this tunnel serves
func (tc *TunnelConnection) GetDomain() string {
	return tc.domain
}

// GetTargetPort returns the target port
func (tc *TunnelConnection) GetTargetPort() int {
	return tc.targetPort
}