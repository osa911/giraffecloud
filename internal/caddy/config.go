// Package caddy provides integration with Caddy server for dynamic reverse proxy configuration.
package caddy

// DefaultAdminEndpoint is the base endpoint for Caddy's admin API
const DefaultAdminEndpoint = "config/"

// CaddyPaths provides standardized paths for Caddy-related files
var CaddyPaths = struct {
	// Config is the path to the Caddy config file
	Config string
}{
	Config: "/etc/caddy/caddy.json",
}

/*
CaddyIntegration Documentation

Overview:
The application integrates with Caddy server for dynamic reverse proxy configuration.
This integration uses HTTP communication over Docker's internal network.

Key Components:
1. HTTP Communication:
   - The application communicates with Caddy through HTTP on 172.20.0.4:2019
   - Communication is secure as it's within Docker's internal network
   - No need for Unix socket or file system access

2. Configuration:
   - Caddy's admin API is used for dynamic configuration
   - The application automatically loads and updates Caddy configuration
   - Routes are managed through the admin API endpoints

Docker Setup Requirements:
1. Network:
   - Caddy and API must be on the same Docker network
   - Caddy must have a fixed IP (172.20.0.4)
   - API must have access to Caddy's admin port (2019)

2. Security:
   - Admin API is only accessible within Docker network
   - No external exposure of admin API
   - All communication is internal and secure

Usage Example:
	caddyService := service.NewCaddyService()
	err := caddyService.LoadConfig()

Common Issues:
1. Connection refused:
   - Check if Caddy container is running
   - Verify network configuration
   - Ensure admin API is listening on correct IP/port

2. Route configuration fails:
   - Check JSON configuration format
   - Verify server block name (srv0)
   - Ensure proper route matching
*/