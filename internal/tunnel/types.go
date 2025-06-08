package tunnel

import (
	"net"
	"sync"
)

// TunnelHandshakeRequest represents the initial handshake message
type TunnelHandshakeRequest struct {
	Token string `json:"token"`
}

// TunnelHandshakeResponse represents the server's response to a handshake
type TunnelHandshakeResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Domain     string `json:"domain,omitempty"`
	TargetPort int    `json:"target_port,omitempty"`
}

// TunnelConnection represents an active tunnel connection with synchronization
type TunnelConnection struct {
	conn       net.Conn    // The underlying network connection
	domain     string      // The domain this tunnel serves
	targetPort int         // The target port on the client side
	mu         sync.Mutex  // Mutex to serialize HTTP request/response cycles
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

// Lock locks the tunnel connection for exclusive access
func (tc *TunnelConnection) Lock() {
	tc.mu.Lock()
}

// Unlock unlocks the tunnel connection
func (tc *TunnelConnection) Unlock() {
	tc.mu.Unlock()
}