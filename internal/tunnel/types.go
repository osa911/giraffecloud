package tunnel

import (
	"net"
)

// TunnelHandshakeRequest represents the initial handshake message
type TunnelHandshakeRequest struct {
	Token  string `json:"token"`
	Domain string `json:"domain"`
	Port   int    `json:"port"`
}

// TunnelHandshakeResponse represents the server's response to a handshake
type TunnelHandshakeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TunnelConnection represents an active tunnel connection
type TunnelConnection struct {
	conn     net.Conn
	domain   string
	stopChan chan struct{}
}