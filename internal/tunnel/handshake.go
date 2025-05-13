package tunnel

import (
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"net"
)

// Perform performs the initial handshake with the server
func Perform(conn net.Conn, token, domain string, port int) (*TunnelHandshakeResponse, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Starting handshake with server at %s", conn.RemoteAddr())

	// Create and send handshake request
	req := TunnelHandshakeRequest{
		Token:  token,
		Domain: domain,
		Port:   port,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		logger.Error("Failed to send handshake request: %v", err)
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	// Read response
	var resp TunnelHandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		logger.Error("Failed to read handshake response: %v", err)
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	if resp.Status != "success" {
		err := fmt.Errorf("handshake failed: %s", resp.Message)
		logger.Error(err.Error())
		return nil, err
	}

	logger.Info("Handshake completed successfully")
	return &resp, nil
}