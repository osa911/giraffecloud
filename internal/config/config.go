package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Server Configuration
	Environment string `env:"ENV" envDefault:"development"`
	ServerIP    string `env:"SERVER_IP"`
	BaseDomain  string `env:"BASE_DOMAIN" envDefault:"giraffecloud.xyz"`
	Port        string `env:"API_PORT" envDefault:"8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"INFO"`
	LogFile     string `env:"LOG_FILE"`
	LogFormat   string `env:"LOG_FORMAT" envDefault:"text"`

	// Database Configuration
	DatabaseURL string `env:"DATABASE_URL"`

	// Tunnel Configuration
	TunnelPort     string `env:"TUNNEL_PORT" envDefault:"4443"`
	GRPCTunnelPort string `env:"GRPC_TUNNEL_PORT" envDefault:"4444"`
	HijackPort     string `env:"HIJACK_PORT" envDefault:"8081"`

	// Caddy Configuration
	CaddyAdminAPI string `env:"CADDY_ADMIN_API" envDefault:"http://localhost:2019"`

	// Client Configuration
	ClientURL string `env:"CLIENT_URL"`

	// Telemetry Configuration
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`

	// Cloudflare Configuration
	CloudflareToken string `env:"CLOUDFLARE_API_TOKEN"`
}

// Load loads the configuration from environment variables and .env files
func Load() (*Config, error) {
	// Load .env file if it exists
	// Try multiple locations for .env file
	envLocations := []string{
		"internal/config/env/.env.production",
		"internal/config/env/.env.development",
		".env",
	}

	// If ENV is set, try to load that specific file first
	envName := os.Getenv("ENV")
	if envName != "" {
		envLocations = append([]string{fmt.Sprintf("internal/config/env/.env.%s", envName)}, envLocations...)
	}

	for _, loc := range envLocations {
		if err := godotenv.Load(loc); err == nil {
			// Found and loaded a file, stop looking unless we want to cascade (usually one is enough)
			// But godotenv.Load doesn't overwrite existing env vars, so it's safe to load multiple?
			// Actually, we usually want the specific one.
			break
		}
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Derive BaseDomain from ClientURL if not explicitly set
	if cfg.BaseDomain == "giraffecloud.xyz" && cfg.ClientURL != "" {
		// Manual parse to avoid circular dependency with utils
		domain := cfg.ClientURL
		domain = strings.TrimPrefix(domain, "https://")
		domain = strings.TrimPrefix(domain, "http://")
		domain = strings.TrimPrefix(domain, "www.")
		if idx := strings.Index(domain, ":"); idx != -1 {
			domain = domain[:idx]
		}
		if domain != "" && domain != "localhost" {
			cfg.BaseDomain = domain
		}
	}

	// Set default log file if not set
	if cfg.LogFile == "" {
		if cfg.Environment == "production" {
			cfg.LogFile = "/app/logs/api.log"
		} else {
			cfg.LogFile = "./logs/api.log"
		}
	}

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return cfg, nil
}
