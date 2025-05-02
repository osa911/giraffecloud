// Package caddy provides integration with Caddy server for dynamic reverse proxy configuration.
package caddy

// CaddyConfig contains all the configuration constants for Caddy integration
const (
	// DefaultSocketPath is the default Unix socket path for Caddy admin API
	// This socket is used for communication between the application and Caddy server
	DefaultSocketPath = "/run/caddy/admin.sock"

	// DefaultConfigPath is the default path for Caddy configuration file
	DefaultConfigPath = "/etc/caddy/Caddyfile"

	// DefaultAdminEndpoint is the base endpoint for Caddy's admin API
	DefaultAdminEndpoint = "/config/"
)

// CaddyPaths provides standardized paths for Caddy-related files
var CaddyPaths = struct {
	// Socket is the path to the Unix socket file
	Socket string
	// Config is the path to the Caddyfile
	Config string
}{
	Socket: DefaultSocketPath,
	Config: DefaultConfigPath,
}

/*
CaddyIntegration Documentation

Overview:
The application integrates with Caddy server for dynamic reverse proxy configuration.
This integration is only active in production mode and uses Unix socket communication.

Key Components:
1. Unix Socket Communication:
   - The application communicates with Caddy through a Unix socket at /run/caddy/admin.sock
   - The socket must be mounted in Docker: /run/caddy/admin.sock:/run/caddy/admin.sock

2. Configuration:
   - Caddy's admin API is used for dynamic configuration
   - No HTTP URL prefix is needed as we communicate directly via Unix socket
   - The application automatically loads and updates Caddy configuration

Docker Setup Requirements:
1. Volume mount for Unix socket:
   volumes:
     - /run/caddy/admin.sock:/run/caddy/admin.sock

2. Socket permissions:
   - The socket file must exist on the host
   - Proper read/write permissions must be set
   - The application container user must have access to the socket

Usage Example:
	caddyService := service.NewCaddyService()
	err := caddyService.LoadConfig()
*/