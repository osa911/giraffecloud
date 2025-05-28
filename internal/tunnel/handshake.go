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

	// Create and send handshake request
	req := TunnelHandshakeRequest{
		Token: token,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	// Read response
	var resp TunnelHandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("handshake failed: %s", resp.Message)
	}

	// Only update config if server provided new values
	if resp.Domain != "" || resp.TargetPort != 0 {
		cfg, err := LoadConfig()
		if err != nil {
			// Don't fail handshake if config update fails
			logger.Info("Handshake completed successfully with domain %s and port %d", resp.Domain, resp.TargetPort)
			return &resp, nil
		}

		// Update only if server provided the values
		if resp.Domain != "" {
			cfg.Domain = resp.Domain
		}
		if resp.TargetPort != 0 {
			cfg.LocalPort = resp.TargetPort
		}

		// Don't fail handshake if config save fails
		_ = SaveConfig(cfg)
	}

	logger.Info("Handshake completed successfully with domain %s and port %d", resp.Domain, resp.TargetPort)
	return &resp, nil
}