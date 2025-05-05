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
	logger := logging.GetGlobalLogger()
	logger.Info("Loading tunnel configuration")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory: %v", err)
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".giraffecloud")
	configPath := filepath.Join(configDir, "tunnel.json")

	// Return default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Info("Config file not found, using default configuration")
		return &DefaultConfig, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		logger.Error("Failed to read config file %s: %v", configPath, err)
		return nil, fmt.Errorf("failed to read tunnel config file: %w", err)
	}
	logger.Info("Read config file: %s", configPath)

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Error("Failed to parse config file: %v", err)
		return nil, fmt.Errorf("failed to parse tunnel config file: %w", err)
	}
	logger.Info("Successfully parsed configuration")

	return &cfg, nil
}

// SaveConfig saves tunnel configuration to the default location
func SaveConfig(cfg *Config) error {
	logger := logging.GetGlobalLogger()
	logger.Info("Saving tunnel configuration")

	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration: %v", err)
		return fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory: %v", err)
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".giraffecloud")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		logger.Error("Failed to create config directory %s: %v", configDir, err)
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	logger.Info("Ensured config directory exists: %s", configDir)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal config: %v", err)
		return fmt.Errorf("failed to marshal tunnel config: %w", err)
	}

	configPath := filepath.Join(configDir, "tunnel.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		logger.Error("Failed to write config file %s: %v", configPath, err)
		return fmt.Errorf("failed to write tunnel config file: %w", err)
	}
	logger.Info("Successfully saved configuration to: %s", configPath)

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	logger := logging.GetGlobalLogger()
	logger.Info("Validating configuration")

	if c.Token == "" {
		logger.Error("Token is required")
		return fmt.Errorf("token is required")
	}

	if c.Server.Host == "" {
		logger.Error("Server host is required")
		return fmt.Errorf("server host is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		logger.Error("Invalid server port: %d", c.Server.Port)
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Local.Host == "" {
		logger.Error("Local host is required")
		return fmt.Errorf("local host is required")
	}

	if c.Local.Port <= 0 || c.Local.Port > 65535 {
		logger.Error("Invalid local port: %d", c.Local.Port)
		return fmt.Errorf("invalid local port: %d", c.Local.Port)
	}

	switch c.Protocol {
	case "http", "https", "tcp":
		// valid protocols
	default:
		logger.Error("Invalid protocol: %s", c.Protocol)
		return fmt.Errorf("invalid protocol: %s", c.Protocol)
	}

	if err := c.Logging.Validate(); err != nil {
		logger.Error("Invalid logging config: %v", err)
		return fmt.Errorf("logging config: %w", err)
	}

	logger.Info("Configuration validation successful")
	return nil
}