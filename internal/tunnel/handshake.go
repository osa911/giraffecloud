package tunnel

import (
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"net"
)

// Perform performs the initial handshake with the server
func Perform(conn net.Conn, token string) (*TunnelHandshakeResponse, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Starting handshake with server at %s", conn.RemoteAddr())

	// Create and send handshake request
	req := TunnelHandshakeRequest{
		Token: token,
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

	// Update config with domain and port from server
	cfg, err := LoadConfig()
	if err != nil {
		logger.Error("Failed to load config: %v", err)
		return &resp, nil // Don't fail the handshake if config update fails
	}

	// Only update if server provided the values
	if resp.Domain != "" {
		cfg.Domain = resp.Domain
	}
	if resp.TargetPort != 0 {
		cfg.LocalPort = resp.TargetPort
	}

	if err := SaveConfig(cfg); err != nil {
		logger.Error("Failed to save config: %v", err)
		return &resp, nil // Don't fail the handshake if config update fails
	}

	logger.Info("Handshake completed successfully with domain %s and port %d", resp.Domain, resp.TargetPort)
	return &resp, nil
}