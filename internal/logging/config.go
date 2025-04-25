package logging

import (
	"fmt"
)

// Config holds logging-related configuration
type Config struct {
	Level      string `json:"level"`       // debug, info, warn, error
	File       string `json:"file"`        // Path to log file
	MaxSize    int    `json:"max_size"`    // Max size in MB
	MaxBackups int    `json:"max_backups"` // Number of backups to keep
	MaxAge     int    `json:"max_age"`     // Max age in days
}

// Validate checks if the configuration is valid (used for CLI)
func (l *Config) Validate() error {
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