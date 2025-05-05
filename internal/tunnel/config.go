package tunnel

import (
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"os"
	"path/filepath"
)

// Server represents a server configuration with host and port
type Server struct {
	Host string `json:"host"` // Host to connect to
	Port int    `json:"port"` // Port number
}

// Config holds tunnel-specific configuration
type Config struct {
	// Server configuration
	Server Server `json:"server"` // Remote server configuration

	// Local service configuration
	Local Server `json:"local"` // Local service configuration

	// Protocol configuration
	Protocol string `json:"protocol"` // Protocol type (http, https, tcp)

	// Security configuration
	Security struct {
		// InsecureSkipVerify should only be used for development/testing
		// When true, the client will not verify the server's certificate
		InsecureSkipVerify bool `json:"insecure_skip_verify"`
	} `json:"security"`

	// Logging configuration
	Logging logging.Config `json:"logging"` // Logging configuration

	// Authentication
	Token string `json:"token"` // Authentication token
}

// DefaultConfig provides default tunnel configuration
var DefaultConfig = Config{
	Server: Server{
		Host: "api.giraffecloud.xyz",
		Port: 443,
	},
	Local: Server{
		Host: "localhost",
		Port: 8080,
	},
	Protocol: "http",
	Security: struct {
		InsecureSkipVerify bool `json:"insecure_skip_verify"`
	}{
		InsecureSkipVerify: false,
	},
	Logging: logging.Config{
		Level:      "info",
		File:       "~/.giraffecloud/tunnel.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	},
}

// LoadConfig loads tunnel configuration from the default location
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".giraffecloud")
	configPath := filepath.Join(configDir, "tunnel.json")

	// Return default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &DefaultConfig, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tunnel config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse tunnel config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves tunnel configuration to the default location
func SaveConfig(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".giraffecloud")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel config: %w", err)
	}

	configPath := filepath.Join(configDir, "tunnel.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tunnel config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}

	if c.Server.Host == "" {
		return fmt.Errorf("server host is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Local.Host == "" {
		return fmt.Errorf("local host is required")
	}

	if c.Local.Port <= 0 || c.Local.Port > 65535 {
		return fmt.Errorf("invalid local port: %d", c.Local.Port)
	}

	switch c.Protocol {
	case "http", "https", "tcp":
		// valid protocols
	default:
		return fmt.Errorf("invalid protocol: %s", c.Protocol)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	return nil
}