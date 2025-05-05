package tunnel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
)

// Handshake message types
const (
	handshakeMsgTypeRequest = iota
	handshakeMsgTypeResponse
)

// Maximum allowed message size (1MB)
const maxMessageSize = 1024 * 1024

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
	logger := logging.GetGlobalLogger()
	logger.Info("Preparing to write handshake message type: %d", msg.Type)

	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal handshake message: %v", err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	logger.Info("Handshake message marshaled, size: %d bytes", len(data))

	// Validate message size
	if len(data) > maxMessageSize {
		err := fmt.Errorf("message too large: %d bytes (max: %d bytes)", len(data), maxMessageSize)
		logger.Error("Invalid message size: %v", err)
		return err
	}

	// Write message length
	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		logger.Error("Failed to write message length: %v", err)
		return fmt.Errorf("failed to write message length: %w", err)
	}
	logger.Info("Wrote message length: %d bytes", length)

	// Write message data
	n, err := conn.Write(data)
	if err != nil {
		logger.Error("Failed to write message data (wrote %d/%d bytes): %v", n, len(data), err)
		return fmt.Errorf("failed to write message data: %w", err)
	}
	if n != len(data) {
		err := fmt.Errorf("incomplete write: wrote %d/%d bytes", n, len(data))
		logger.Error("Failed to write complete message: %v", err)
		return err
	}
	logger.Info("Successfully wrote handshake message (%d bytes)", n)

	return nil
}

// readHandshakeMessage reads a handshake message from the connection
func readHandshakeMessage(conn net.Conn) (*handshakeMessage, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Starting to read handshake message")

	// Read message length
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		if err == io.EOF {
			logger.Info("Connection closed while reading message length (EOF)")
			return nil, err
		}
		logger.Error("Failed to read message length: %v", err)
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	logger.Info("Read message length: %d bytes", length)

	// Validate message size
	if length > maxMessageSize {
		err := fmt.Errorf("message too large: %d bytes (max: %d bytes)", length, maxMessageSize)
		logger.Error("Invalid message size: %v", err)
		return nil, err
	}

	// Read message data
	data := make([]byte, length)
	n, err := io.ReadFull(conn, data)
	if err != nil {
		logger.Error("Failed to read message data (read %d/%d bytes): %v", n, length, err)
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}
	logger.Info("Read message data successfully (%d bytes)", n)

	// Unmarshal message
	var msg handshakeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("Failed to unmarshal message: %v", err)
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	logger.Info("Successfully unmarshaled handshake message of type: %d", msg.Type)

	return &msg, nil
}

// Perform performs the initial handshake with the server
func Perform(conn net.Conn, token string) (*handshakeResponse, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Starting handshake with server at %s", conn.RemoteAddr())

	// Create handshake request
	req := handshakeRequest{Token: token}
	reqData, err := json.Marshal(req)
	if err != nil {
		logger.Error("Failed to marshal handshake request: %v", err)
		return nil, fmt.Errorf("failed to marshal handshake request: %w", err)
	}
	logger.Info("Created handshake request (size: %d bytes)", len(reqData))

	// Send handshake message
	msg := &handshakeMessage{
		Type:    handshakeMsgTypeRequest,
		Payload: reqData,
	}
	if err := writeHandshakeMessage(conn, msg); err != nil {
		logger.Error("Failed to send handshake message: %v", err)
		return nil, fmt.Errorf("failed to send handshake message: %w", err)
	}
	logger.Info("Sent handshake request to server")

	// Read response
	resp, err := readHandshakeMessage(conn)
	if err != nil {
		logger.Error("Failed to read handshake response: %v", err)
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}
	logger.Info("Received handshake response from server")

	if resp.Type != handshakeMsgTypeResponse {
		err := fmt.Errorf("unexpected message type in handshake response: %d", resp.Type)
		logger.Error(err.Error())
		return nil, err
	}

	// Parse handshake response
	var handshakeResp handshakeResponse
	if err := json.Unmarshal(resp.Payload, &handshakeResp); err != nil {
		logger.Error("Failed to unmarshal handshake response: %v", err)
		return nil, fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}
	logger.Info("Parsed handshake response: status=%s, message=%s", handshakeResp.Status, handshakeResp.Message)

	if handshakeResp.Status != "success" {
		err := fmt.Errorf("handshake failed: %s", handshakeResp.Message)
		logger.Error(err.Error())
		return nil, err
	}

	logger.Info("Handshake completed successfully")
	return &handshakeResp, nil
}