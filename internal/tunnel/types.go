package tunnel

import (
	"net"
	"time"
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

// PingMessage represents a ping request
type PingMessage struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
}

// PongMessage represents a pong response
type PongMessage struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	RTT       int64  `json:"rtt"`
}

// TunnelConnection represents an active tunnel connection
type TunnelConnection struct {
	conn       net.Conn
	domain     string
	targetPort int
	stopChan   chan struct{}
	lastPing   time.Time
}