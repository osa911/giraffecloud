package tunnel

import (
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"net"
)

// Perform performs the initial handshake with the server (backward compatibility)
// For new code, use the performHandshake method in the Tunnel struct
func Perform(conn net.Conn, token string) (*TunnelHandshakeResponse, error) {
	logger := logging.GetGlobalLogger()

	// Default to HTTP connection type for backward compatibility
	return performHandshakeWithType(conn, token, "http", logger)
}

// PerformWithType performs handshake with specified connection type
func PerformWithType(conn net.Conn, token, connType string) (*TunnelHandshakeResponse, error) {
	logger := logging.GetGlobalLogger()
	return performHandshakeWithType(conn, token, connType, logger)
}

// performHandshakeWithType is the internal implementation
func performHandshakeWithType(conn net.Conn, token, connType string, logger *logging.Logger) (*TunnelHandshakeResponse, error) {
	// Create and send handshake request with connection type
	req := TunnelHandshakeRequest{
		Token:          token,
		ConnectionType: connType,
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

	// Only update config if server provided new values (backward compatibility)
	if resp.Domain != "" || resp.TargetPort != 0 {
		cfg, err := LoadConfig()
		if err != nil {
			// Don't fail handshake if config update fails
			logger.Info("Handshake completed successfully with domain %s and port %d (type: %s)", resp.Domain, resp.TargetPort, connType)
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

	logger.Info("Handshake completed successfully with domain %s and port %d (type: %s)", resp.Domain, resp.TargetPort, connType)
	return &resp, nil
}