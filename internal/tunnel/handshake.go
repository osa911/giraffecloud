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
	Status    string `json:"status"`
	Message   string `json:"message"`
	Domain    string `json:"domain"`
	LocalPort int    `json:"local_port"`
	Protocol  string `json:"protocol"`
}

// writeHandshakeMessage writes a handshake message to the connection
func writeHandshakeMessage(conn net.Conn, msg *handshakeMessage) error {
	logger := logging.GetGlobalLogger()
	logger.Info("Preparing to write handshake message type: %d", msg.Type)

	// Marshal message to JSON
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

	// Prepare length bytes
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))

	// Write length and data in a single buffer
	buffer := append(lengthBytes, data...)
	n, err := conn.Write(buffer)
	if err != nil {
		logger.Error("Failed to write message (wrote %d/%d bytes): %v", n, len(buffer), err)
		return fmt.Errorf("failed to write message: %w", err)
	}
	if n != len(buffer) {
		err := fmt.Errorf("incomplete write: wrote %d/%d bytes", n, len(buffer))
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

	// Read message length (4 bytes)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		if err == io.EOF {
			logger.Info("Connection closed while reading message length (EOF)")
			return nil, err
		}
		logger.Error("Failed to read message length: %v", err)
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Convert bytes to uint32
	length := binary.BigEndian.Uint32(lengthBytes)
	logger.Info("Read message length: %d bytes", length)

	// Validate message size
	if length > maxMessageSize {
		err := fmt.Errorf("message too large: %d bytes (max: %d bytes)", length, maxMessageSize)
		logger.Error("Invalid message size: %v", err)
		return nil, err
	}

	// Read message data with a separate buffer
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