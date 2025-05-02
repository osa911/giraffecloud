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
	DefaultAdminEndpoint = "config/"
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
   - The Unix socket URL format is: http+unix:///run/caddy/admin.sock

2. Configuration:
   - Caddy's admin API is used for dynamic configuration
   - The proper URL scheme for Unix socket is "http+unix://"
   - The application automatically loads and updates Caddy configuration

Docker Setup Requirements:
1. Volume mount for Unix socket:
   volumes:
     - /run/caddy/admin.sock:/run/caddy/admin.sock

2. Socket permissions:
   - The socket file must exist on the host
   - Proper read/write permissions must be set (usually 660 or 666)
   - The application container user must have access to the socket

Usage Example:
	caddyService := service.NewCaddyService() // Uses CaddyPaths.SocketURL internally
	err := caddyService.LoadConfig()

Common Issues:
1. "unsupported protocol scheme" error:
   - This means the Unix socket URL is not properly formatted
   - Make sure to use the complete URL: http+unix:///run/caddy/admin.sock
   - Use CaddyPaths.SocketURL which has the correct format

2. Permission denied:
   - Check socket file permissions: sudo chmod 666 /run/caddy/admin.sock
   - Verify Docker volume mount is correct
   - Ensure container user has proper permissions
*/