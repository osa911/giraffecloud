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

// Config represents the tunnel client configuration
type Config struct {
	Token     string         `json:"token"`
	Domain    string         `json:"domain"`
	LocalPort int           `json:"local_port"`
	Server    ServerConfig   `json:"server"`
	API       ServerConfig   `json:"api"`
	Security  SecurityConfig `json:"security"`
}

// ServerConfig represents server connection settings
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// SecurityConfig represents security settings
type SecurityConfig struct {
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACert             string `json:"ca_cert"`
	ClientCert         string `json:"client_cert"`
	ClientKey          string `json:"client_key"`
}

// DefaultConfig provides default tunnel configuration
var DefaultConfig = Config{
	Server: ServerConfig{
		Host: "tunnel.giraffecloud.xyz",
		Port: 4443,
	},
	API: ServerConfig{
		Host: "api.giraffecloud.xyz",
		Port: 443,
	},
	Security: SecurityConfig{
		InsecureSkipVerify: false,
	},
}

// LoadConfig loads the configuration from the default location
func LoadConfig() (*Config, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Loading tunnel configuration")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory: %v", err)
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".giraffecloud", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Config file not found, using default configuration")
			return &DefaultConfig, nil
		}
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

// SaveConfig saves the configuration to the default location
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

	configPath := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal config: %v", err)
		return fmt.Errorf("failed to marshal tunnel config: %w", err)
	}

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

	if c.API.Host == "" {
		logger.Error("API host is required")
		return fmt.Errorf("api host is required")
	}

	if c.API.Port <= 0 || c.API.Port > 65535 {
		logger.Error("Invalid API port: %d", c.API.Port)
		return fmt.Errorf("invalid api port: %d", c.API.Port)
	}

	if c.Security.InsecureSkipVerify {
		logger.Warn("InsecureSkipVerify is true, the client will not verify the server's certificate")
	}

	logger.Info("Configuration validation successful")
	return nil
}