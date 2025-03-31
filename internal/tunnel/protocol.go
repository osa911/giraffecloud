package tunnel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

const (
	// Protocol version
	ProtocolVersion = 1

	// Message types
	MsgTypeHandshake = 0x01
	MsgTypeData      = 0x02
	MsgTypeClose     = 0x03
	MsgTypeError     = 0x04

	// Handshake status codes
	HandshakeSuccess = 0x00
	HandshakeError   = 0x01
)

// Message represents a protocol message
type Message struct {
	Type    byte
	Length  uint32
	Payload []byte
}

// WriteMessage writes a message to the connection
func WriteMessage(conn net.Conn, msg *Message) error {
	// Write message header (1 byte type + 4 bytes length)
	header := make([]byte, 5)
	header[0] = msg.Type
	binary.BigEndian.PutUint32(header[1:], uint32(len(msg.Payload)))

	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("failed to write message header: %w", err)
	}

	// Write payload if any
	if len(msg.Payload) > 0 {
		if _, err := conn.Write(msg.Payload); err != nil {
			return fmt.Errorf("failed to write message payload: %w", err)
		}
	}

	return nil
}

// ReadMessage reads a message from the connection
func ReadMessage(conn net.Conn) (*Message, error) {
	// Read message header
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	msg := &Message{
		Type:   header[0],
		Length: binary.BigEndian.Uint32(header[1:]),
	}

	// Read payload if any
	if msg.Length > 0 {
		msg.Payload = make([]byte, msg.Length)
		if _, err := io.ReadFull(conn, msg.Payload); err != nil {
			return nil, fmt.Errorf("failed to read message payload: %w", err)
		}
	}

	return msg, nil
}

// HandshakeRequest represents the initial handshake request
type HandshakeRequest struct {
	Version     byte
	Token       string
	Endpoints   []EndpointInfo
}

// HandshakeResponse represents the server's handshake response
type HandshakeResponse struct {
	Status  byte
	Message string
}

// EndpointInfo contains information about a tunnel endpoint
type EndpointInfo struct {
	Name     string
	Protocol string
	Local    string
	Remote   string
}

// PerformHandshake performs the initial handshake with the server
func PerformHandshake(conn net.Conn, token string, endpoints []EndpointInfo) error {
	// Create handshake request
	req := &HandshakeRequest{
		Version:   ProtocolVersion,
		Token:     token,
		Endpoints: endpoints,
	}

	// Marshal request
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal handshake request: %w", err)
	}

	// Send handshake request
	msg := &Message{
		Type:    MsgTypeHandshake,
		Payload: payload,
	}

	if err := WriteMessage(conn, msg); err != nil {
		return err
	}

	// Read handshake response
	respMsg, err := ReadMessage(conn)
	if err != nil {
		return err
	}

	if respMsg.Type != MsgTypeHandshake {
		return fmt.Errorf("unexpected message type: %x", respMsg.Type)
	}

	var resp HandshakeResponse
	if err := json.Unmarshal(respMsg.Payload, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}

	if resp.Status != HandshakeSuccess {
		return fmt.Errorf("handshake failed: %s", resp.Message)
	}

	return nil
}