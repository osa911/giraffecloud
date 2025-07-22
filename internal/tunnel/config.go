package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

// StreamingConfig holds configuration for streaming optimizations
type StreamingConfig struct {
	// Buffer sizes
	MediaBufferSize    int           `json:"media_buffer_size"`    // Buffer size for media streaming (bytes)
	RegularBufferSize  int           `json:"regular_buffer_size"`  // Buffer size for regular requests (bytes)

	// Connection pool settings
	PoolSize           int           `json:"pool_size"`           // Maximum connections per pool
	PoolTimeout        time.Duration `json:"pool_timeout"`        // Connection timeout
	PoolKeepAlive      time.Duration `json:"pool_keep_alive"`     // Keep-alive duration

	// Timeout settings
	MediaTimeout       time.Duration `json:"media_timeout"`       // Timeout for media requests
	RegularTimeout     time.Duration `json:"regular_timeout"`     // Timeout for regular requests

	// Media detection settings
	EnableMediaOptimization bool     `json:"enable_media_optimization"` // Enable media-specific optimizations
	MediaExtensions        []string `json:"media_extensions"`           // File extensions to treat as media
	MediaPaths             []string `json:"media_paths"`                // URL paths to treat as media

	// Performance settings
	ConcurrentMediaStreams int `json:"concurrent_media_streams"` // Max concurrent media streams per domain
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

// DefaultStreamingConfig returns default streaming configuration
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		MediaBufferSize:   65536, // 64KB
		RegularBufferSize: 32768, // 32KB

		PoolSize:      3,  // Small pool size to prevent connection corruption
		PoolTimeout:   10 * time.Second,
		PoolKeepAlive: 30 * time.Second,

		// Timeout settings - more aggressive to prevent stuck connections
		MediaTimeout:   15 * time.Second,  // 15 seconds for media (aggressive)
		RegularTimeout: 10 * time.Second,  // 10 seconds for regular requests (aggressive)

		EnableMediaOptimization: true,
		MediaExtensions: []string{
			".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".ogg", ".ogv",
			".mp3", ".wav", ".flac", ".aac", ".m4a", ".wma",
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg",
			".pdf", ".zip", ".rar", ".tar", ".gz", ".7z",
		},
		MediaPaths: []string{
			"/video/", "/media/", "/stream/", "/download/", "/files/",
			"/uploads/", "/assets/", "/static/", "/public/",
		},

		ConcurrentMediaStreams: 5,
	}
}

// LoadConfig loads the configuration from the default location
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".giraffecloud", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DefaultConfig, nil
		}
		return nil, fmt.Errorf("failed to read tunnel config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse tunnel config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the default location
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

	configPath := filepath.Join(configDir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel config: %w", err)
	}

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

	if c.API.Host == "" {
		return fmt.Errorf("api host is required")
	}

	if c.API.Port <= 0 || c.API.Port > 65535 {
		return fmt.Errorf("invalid api port: %d", c.API.Port)
	}

	return nil
}

// IsMediaExtension checks if a file extension is considered media
func (c *StreamingConfig) IsMediaExtension(ext string) bool {
	for _, mediaExt := range c.MediaExtensions {
		if ext == mediaExt {
			return true
		}
	}
	return false
}

// IsMediaPath checks if a URL path is considered media
func (c *StreamingConfig) IsMediaPath(path string) bool {
	for _, mediaPath := range c.MediaPaths {
		if len(path) >= len(mediaPath) && path[:len(mediaPath)] == mediaPath {
			return true
		}
	}
	return false
}