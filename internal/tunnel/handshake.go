package tunnel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// Handshake message types
const (
	handshakeMsgTypeRequest = iota
	handshakeMsgTypeResponse
)

// handshakeMessage represents a handshake protocol message
type handshakeMessage struct {
	Type    uint8  `json:"type"`
	Payload []byte `json:"payload"`
}

// handshakeRequest represents the initial handshake message
type handshakeRequest struct {
	Token string `json:"token"`
}

// handshakeResponse represents the server's response to a handshake
type handshakeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// writeHandshakeMessage writes a handshake message to the connection
func writeHandshakeMessage(conn net.Conn, msg *handshakeMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message length
	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message data
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// readHandshakeMessage reads a handshake message from the connection
func readHandshakeMessage(conn net.Conn) (*handshakeMessage, error) {
	// Read message length
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal message
	var msg handshakeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// Perform performs the initial handshake with the server
func Perform(conn net.Conn, token string) (*handshakeResponse, error) {
	// Create handshake request
	req := handshakeRequest{Token: token}
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal handshake request: %w", err)
	}

	// Send handshake message
	msg := &handshakeMessage{
		Type:    handshakeMsgTypeRequest,
		Payload: reqData,
	}
	if err := writeHandshakeMessage(conn, msg); err != nil {
		return nil, fmt.Errorf("failed to send handshake message: %w", err)
	}

	// Read response
	resp, err := readHandshakeMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	if resp.Type != handshakeMsgTypeResponse {
		return nil, fmt.Errorf("unexpected message type in handshake response: %d", resp.Type)
	}

	// Parse handshake response
	var handshakeResp handshakeResponse
	if err := json.Unmarshal(resp.Payload, &handshakeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}

	if handshakeResp.Status != "success" {
		return nil, fmt.Errorf("handshake failed: %s", handshakeResp.Message)
	}

	return &handshakeResp, nil
}