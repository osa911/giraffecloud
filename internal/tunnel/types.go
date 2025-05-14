package tunnel

import (
	"encoding/json"
	"net"
	"sync"
	"time"
)

const (
	// Message types
	MessageTypePing     = "ping"
	MessageTypePong     = "pong"
	MessageTypeData     = "data"
	MessageTypeControl  = "control"
)

// TunnelMessage represents a message sent through the tunnel
type TunnelMessage struct {
	Type    string          `json:"type"`              // Message type (ping, pong, data, control)
	ID      string          `json:"id"`                // Unique message ID for correlation
	Payload json.RawMessage `json:"payload,omitempty"` // Message payload
}

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
	Timestamp int64 `json:"timestamp"` // Timestamp in nanoseconds
}

// PongMessage represents a pong response
type PongMessage struct {
	Timestamp int64 `json:"timestamp"` // Original ping timestamp
	RTT       int64 `json:"rtt"`       // Round trip time in nanoseconds
}

// DataMessage represents a data payload
type DataMessage struct {
	Data []byte `json:"data"` // The actual data being transferred
}

// MessageCorrelation tracks request/response message pairs
type MessageCorrelation struct {
	RequestTime  time.Time
	ResponseChan chan *TunnelMessage
	Timeout     time.Duration
}

// TunnelConnection represents an active tunnel connection
type TunnelConnection struct {
	conn           net.Conn          // The underlying network connection
	domain         string            // The domain this tunnel serves
	targetPort     int              // The target port on the client side
	stopChan       chan struct{}     // Channel to signal connection stop
	lastPing       time.Time         // Time of last successful ping
	reader         *json.Decoder     // JSON decoder for reading messages
	writer         *json.Encoder     // JSON encoder for writing messages
	readerMu       sync.Mutex        // Mutex for synchronizing reader access
	writerMu       sync.Mutex        // Mutex for synchronizing writer access
	correlationMap sync.Map          // Thread-safe map for message correlation
	cleanupTicker  *time.Ticker     // Ticker for cleaning up stale correlations
}