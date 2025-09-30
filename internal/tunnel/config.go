package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

// Server represents a server configuration with host and port
type Server struct {
	Host string `json:"host"` // Host to connect to
	Port int    `json:"port"` // Port number
}

// Config represents the tunnel client configuration
type Config struct {
	Token      string           `json:"token"`
	Domain     string           `json:"domain"`
	LocalPort  int              `json:"local_port"`
	Server     ServerConfig     `json:"server"`
	API        ServerConfig     `json:"api"`
	Security   SecurityConfig   `json:"security"`
	AutoUpdate AutoUpdateConfig `json:"auto_update"`
	TestMode   TestModeConfig   `json:"test_mode"`
}

// TestModeConfig represents test mode settings
type TestModeConfig struct {
	Enabled bool     `json:"enabled"`           // Enable test mode
	Channel string   `json:"channel"`           // Release channel: stable, beta, test
	UserID  string   `json:"user_id,omitempty"` // User ID for test targeting
	Groups  []string `json:"groups,omitempty"`  // Test groups this client belongs to
}

// AutoUpdateConfig represents auto-update settings
type AutoUpdateConfig struct {
	Enabled            bool          `json:"enabled"`                 // Enable automatic updates
	CheckInterval      time.Duration `json:"check_interval"`          // How often to check for updates
	RequiredOnly       bool          `json:"required_only"`           // Only install required updates automatically
	DownloadURL        string        `json:"download_url"`            // Base URL for downloads
	PreserveConnection bool          `json:"preserve_connection"`     // Try to preserve connections during updates
	RestartService     bool          `json:"restart_service"`         // Restart service after update
	BackupCount        int           `json:"backup_count"`            // Number of backups to keep
	UpdateWindow       *TimeWindow   `json:"update_window,omitempty"` // Time window for automatic updates
	Channel            string        `json:"channel"`                 // Release channel override
}

// TimeWindow represents a time window for updates
type TimeWindow struct {
	StartHour int    `json:"start_hour"` // Hour to start updates (0-23)
	EndHour   int    `json:"end_hour"`   // Hour to stop updates (0-23)
	Timezone  string `json:"timezone"`   // Timezone for the time window
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
	MediaBufferSize   int `json:"media_buffer_size"`   // Buffer size for media streaming (bytes)
	RegularBufferSize int `json:"regular_buffer_size"` // Buffer size for regular requests (bytes)

	// Connection pool settings
	PoolSize      int           `json:"pool_size"`       // Maximum connections per pool
	PoolTimeout   time.Duration `json:"pool_timeout"`    // Connection timeout
	PoolKeepAlive time.Duration `json:"pool_keep_alive"` // Keep-alive duration

	// Timeout settings
	MediaTimeout   time.Duration `json:"media_timeout"`   // Timeout for media requests
	RegularTimeout time.Duration `json:"regular_timeout"` // Timeout for regular requests

	// Media detection settings
	EnableMediaOptimization bool     `json:"enable_media_optimization"` // Enable media-specific optimizations
	MediaExtensions         []string `json:"media_extensions"`          // File extensions to treat as media
	MediaPaths              []string `json:"media_paths"`               // URL paths to treat as media

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
		InsecureSkipVerify: false, // PRODUCTION: NEVER skip verification
		// Certificate paths are set dynamically by the 'login' command
		// after fetching certificates from the API server
	},
	AutoUpdate: AutoUpdateConfig{
		Enabled:            true,           // Enable auto-updates by default
		CheckInterval:      24 * time.Hour, // Check daily
		RequiredOnly:       true,           // Only auto-install required updates
		DownloadURL:        "https://github.com/osa911/giraffecloud/releases/download",
		PreserveConnection: true,     // Try to preserve connections
		RestartService:     true,     // Restart service after update
		BackupCount:        5,        // Keep 5 backups
		Channel:            "stable", // Default to stable releases
		UpdateWindow: &TimeWindow{ // Update during off-peak hours
			StartHour: 2, // 2 AM
			EndHour:   6, // 6 AM
			Timezone:  "UTC",
		},
	},
	TestMode: TestModeConfig{
		Enabled: false,      // Test mode disabled by default
		Channel: "stable",   // Default to stable channel
		UserID:  "",         // No user ID by default
		Groups:  []string{}, // No test groups by default
	},
}

// GetConfigDir returns the directory where GiraffeCloud stores config files
func GetConfigDir() (string, error) {
	// Highest priority: explicit override
	if cfgHome := os.Getenv("GIRAFFECLOUD_HOME"); cfgHome != "" {
		return cfgHome, nil
	}

	// Try to get home directory with fallback for problematic environments
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback strategies for environments where UserHomeDir() fails
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, ".giraffecloud"), nil
		}
		// Last resort: use /tmp for service environments
		return filepath.Join("/tmp", "giraffecloud"), nil
	}
	return filepath.Join(homeDir, ".giraffecloud"), nil
}

// GetConfigPath returns the full path to the config.json
func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// EnsureConsistentConfigHome sets GIRAFFECLOUD_HOME to the original sudo user's home when running as root with sudo
// This keeps CLI behavior consistent with the non-root user's config path even when prefixed with sudo.
func EnsureConsistentConfigHome() {
	// Defensive: handle panics in this function
	defer func() {
		if r := recover(); r != nil {
			// If anything panics in this function, just continue silently
			// This prevents segfaults during initialization
		}
	}()

	if os.Getenv("GIRAFFECLOUD_HOME") != "" {
		return
	}
	// Only relevant for Unix-like systems
	if runtime.GOOS == "windows" {
		return
	}
	// If running as root via sudo, prefer the invoking user's home
	if os.Geteuid() == 0 {
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			// Safe usage: check both error and nil before using result
			if u, err := user.Lookup(sudoUser); err == nil && u != nil && u.HomeDir != "" {
				_ = os.Setenv("GIRAFFECLOUD_HOME", filepath.Join(u.HomeDir, ".giraffecloud"))
				// Also force HOME so any other code relying on HOME is consistent
				_ = os.Setenv("HOME", u.HomeDir)
			}
		}
	}
}

// applyAutoUpdateDefaults ensures sensible defaults when fields are zero-valued in existing configs
func applyAutoUpdateDefaults(cfg *Config) {
	// If DownloadURL is empty, apply default
	if cfg.AutoUpdate.DownloadURL == "" {
		cfg.AutoUpdate.DownloadURL = DefaultConfig.AutoUpdate.DownloadURL
	}
	// If CheckInterval is zero, apply default
	if cfg.AutoUpdate.CheckInterval == 0 {
		cfg.AutoUpdate.CheckInterval = DefaultConfig.AutoUpdate.CheckInterval
	}
	// RestartService default should be true when not explicitly set
	if !cfg.AutoUpdate.RestartService {
		// If user explicitly disabled it, we can't distinguish from zero-value.
		// Heuristic: if other fields look zero-ish, treat as backfill.
		if cfg.AutoUpdate.BackupCount == 0 && !cfg.AutoUpdate.PreserveConnection && cfg.AutoUpdate.DownloadURL == DefaultConfig.AutoUpdate.DownloadURL {
			cfg.AutoUpdate.RestartService = true
		}
	}
	// BackupCount default
	if cfg.AutoUpdate.BackupCount == 0 {
		cfg.AutoUpdate.BackupCount = DefaultConfig.AutoUpdate.BackupCount
	}
	// Channel default
	if cfg.AutoUpdate.Channel == "" {
		cfg.AutoUpdate.Channel = DefaultConfig.AutoUpdate.Channel
	}
}

// DefaultStreamingConfig returns default streaming configuration
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		MediaBufferSize:   65536, // 64KB
		RegularBufferSize: 32768, // 32KB

		// HYBRID TUNNEL APPROACH - Hot pool + On-demand creation
		// Hot pool: 10 ready connections for instant response (increased for stability)
		// On-demand: Unlimited additional connections created as needed
		// Less aggressive cleanup: Keep connections longer to maintain pool stability
		PoolSize:      10, // INCREASED: Larger hot pool for better stability
		PoolTimeout:   10 * time.Second,
		PoolKeepAlive: 30 * time.Second,

		// Timeout settings - more aggressive to prevent stuck connections
		MediaTimeout:   15 * time.Second, // 15 seconds for media (aggressive)
		RegularTimeout: 10 * time.Second, // 10 seconds for regular requests (aggressive)

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
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
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

	// Backfill defaults for configs created by older versions (missing auto_update fields)
	applyAutoUpdateDefaults(&cfg)

	return &cfg, nil
}

// MergeConfig merges changes from new config into existing config
func MergeConfig(existing, new *Config) *Config {
	// If no existing config, start with default
	if existing == nil {
		existing = &DefaultConfig
	}

	// Create a copy to avoid modifying the input
	merged := *existing

	// Merge changes from new config
	if new.Token != "" {
		merged.Token = new.Token
	}
	if new.Domain != "" {
		merged.Domain = new.Domain
	}
	if new.LocalPort != 0 {
		merged.LocalPort = new.LocalPort
	}
	if new.Server.Host != "" {
		merged.Server.Host = new.Server.Host
	}
	if new.Server.Port != 0 {
		merged.Server.Port = new.Server.Port
	}
	if new.API.Host != "" {
		merged.API.Host = new.API.Host
	}
	if new.API.Port != 0 {
		merged.API.Port = new.API.Port
	}

	// Always update test mode and auto-update settings
	merged.TestMode = new.TestMode
	merged.AutoUpdate = new.AutoUpdate

	// Security settings are only updated when explicitly set
	if new.Security.CACert != "" {
		merged.Security.CACert = new.Security.CACert
	}
	if new.Security.ClientCert != "" {
		merged.Security.ClientCert = new.Security.ClientCert
	}
	if new.Security.ClientKey != "" {
		merged.Security.ClientKey = new.Security.ClientKey
	}

	return &merged
}

// SaveConfig saves the configuration to the default location
func SaveConfig(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid tunnel configuration: %w", err)
	}

	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
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

// ResolveReleaseChannel returns the effective release channel for this client.
// Priority: TestMode.Channel (when enabled) > AutoUpdate.Channel > "" (default server-side behavior)
func ResolveReleaseChannel() string {
	cfg, err := LoadConfig()
	if err == nil && cfg != nil {
		if cfg.TestMode.Enabled && cfg.TestMode.Channel != "" {
			return cfg.TestMode.Channel
		}
		if cfg.AutoUpdate.Channel != "" {
			return cfg.AutoUpdate.Channel
		}
	}
	return ""
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
