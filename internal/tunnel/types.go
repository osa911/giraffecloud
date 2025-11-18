package tunnel

import (
	"net"
	"sync"
	"time"
)

// TunnelHandshakeRequest represents the initial handshake message
type TunnelHandshakeRequest struct {
	Token          string `json:"token"`
	Domain         string `json:"domain,omitempty"`          // For multi-tunnel support
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

// UsageRecorder is a lightweight interface for recording usage stats.
type UsageRecorder interface {
	Increment(userID uint32, tunnelID uint32, domain string, bytesIn int64, bytesOut int64, requests int64)
}

// TunnelConnection represents an active tunnel connection with per-connection synchronization
// Each connection maintains HTTP/1.1 request-response ordering while the pool enables concurrency
type TunnelConnection struct {
	conn         net.Conn   // The underlying network connection
	domain       string     // The domain this tunnel serves
	targetPort   int        // The target port on the client side
	mu           sync.Mutex // Mutex to serialize HTTP request/response cycles PER CONNECTION
	requestCount int64      // Number of requests handled by this connection
	createdAt    time.Time  // When this connection was created
	// Pool-level concurrency is achieved by having multiple connections
}

// NewTunnelConnection creates a new tunnel connection
func NewTunnelConnection(domain string, conn net.Conn, targetPort int) *TunnelConnection {
	return &TunnelConnection{
		conn:         conn,
		domain:       domain,
		targetPort:   targetPort,
		requestCount: 0,
		createdAt:    time.Now(),
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

// Lock locks this specific tunnel connection for exclusive HTTP request-response cycle
func (tc *TunnelConnection) Lock() {
	tc.mu.Lock()
}

// Unlock unlocks this specific tunnel connection
func (tc *TunnelConnection) Unlock() {
	tc.mu.Unlock()
}

// GetRequestCount returns the number of requests handled by this connection
func (tc *TunnelConnection) GetRequestCount() int64 {
	return tc.requestCount
}

// GetCreatedAt returns when this connection was created
func (tc *TunnelConnection) GetCreatedAt() time.Time {
	return tc.createdAt
}

// IncrementRequestCount increments the request counter for this connection
func (tc *TunnelConnection) IncrementRequestCount() {
	tc.requestCount++
}
