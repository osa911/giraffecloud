package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Token     string            `yaml:"token"`
	Endpoints []EndpointConfig  `yaml:"endpoints"`
	Server    ServerConfig      `yaml:"server"`
	Logging   LoggingConfig     `yaml:"logging"`
	Security  SecurityConfig    `yaml:"security"`
}

type EndpointConfig struct {
	Name     string `yaml:"name"`
	Local    string `yaml:"local"`
	Remote   string `yaml:"remote"`
	Protocol string `yaml:"protocol"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	File       string `yaml:"file"`        // Path to log file
	MaxSize    int    `yaml:"max_size"`    // Max size in MB
	MaxBackups int    `yaml:"max_backups"` // Number of backups to keep
	MaxAge     int    `yaml:"max_age"`     // Max age in days
}

type SecurityConfig struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // Skip TLS verification
	CertFile          string `yaml:"cert_file"`             // Path to client cert
	KeyFile           string `yaml:"key_file"`              // Path to client key
	CAFile            string `yaml:"ca_file"`               // Path to CA cert
}

var DefaultConfig = Config{
	Server: ServerConfig{
		Host: "api.giraffecloud.com",
		Port: 443,
	},
	Logging: LoggingConfig{
		Level:      "info",
		File:       "~/.giraffecloud/tunnel.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	},
	Security: SecurityConfig{
		InsecureSkipVerify: false,
	},
}

const (
	configDir  = ".giraffecloud"
	configFile = "config"
)

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDirPath := filepath.Join(homeDir, configDir)
	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDirPath, configFile), nil
}

func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}

	if len(c.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint is required")
	}

	for i, ep := range c.Endpoints {
		if err := ep.Validate(); err != nil {
			return fmt.Errorf("endpoint %d: %w", i, err)
		}
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	if err := c.Security.Validate(); err != nil {
		return fmt.Errorf("security config: %w", err)
	}

	return nil
}

func (ep *EndpointConfig) Validate() error {
	if ep.Name == "" {
		return fmt.Errorf("name is required")
	}

	if ep.Local == "" {
		return fmt.Errorf("local address is required")
	}

	if ep.Remote == "" {
		return fmt.Errorf("remote address is required")
	}

	if ep.Protocol == "" {
		return fmt.Errorf("protocol is required")
	}

	// Validate protocol
	validProtocols := map[string]bool{
		"http":  true,
		"https": true,
		"tcp":   true,
		"udp":   true,
	}

	if !validProtocols[ep.Protocol] {
		return fmt.Errorf("invalid protocol: %s", ep.Protocol)
	}

	// Validate local address format
	if !strings.Contains(ep.Local, ":") {
		return fmt.Errorf("invalid local address format: %s", ep.Local)
	}

	return nil
}

func (s *ServerConfig) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}

	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("invalid port: %d", s.Port)
	}

	return nil
}

func (l *LoggingConfig) Validate() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[l.Level] {
		return fmt.Errorf("invalid log level: %s", l.Level)
	}

	if l.MaxSize <= 0 {
		return fmt.Errorf("max_size must be positive")
	}

	if l.MaxBackups < 0 {
		return fmt.Errorf("max_backups must be non-negative")
	}

	if l.MaxAge < 0 {
		return fmt.Errorf("max_age must be non-negative")
	}

	return nil
}

func (s *SecurityConfig) Validate() error {
	if s.CertFile != "" && s.KeyFile == "" {
		return fmt.Errorf("key_file is required when cert_file is specified")
	}

	if s.KeyFile != "" && s.CertFile == "" {
		return fmt.Errorf("cert_file is required when key_file is specified")
	}

	return nil
}

// LoadConfig loads the configuration from disk
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to disk
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}