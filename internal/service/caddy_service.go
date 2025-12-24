package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/osa911/giraffecloud/internal/config"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/tunnel"
)

// CaddyService defines the interface for Caddy operations
type CaddyService interface {
	ConfigureRoute(domain string, targetIP string, targetPort int) error
	RemoveRoute(domain string) error
	ValidateConnection() error
	LoadConfig() error
	ConfigureTunnelRoute(domain string, targetIP string, targetPort int) error
	RemoveTunnelRoute(domain string) error
}

type caddyService struct {
	logger      *logging.Logger
	client      *http.Client
	baseURL     string
	mu          sync.RWMutex
	connections *tunnel.ConnectionManager
	config      *config.Config
}

// NewCaddyService creates a new Caddy service instance
func NewCaddyService(connections *tunnel.ConnectionManager, cfg *config.Config) CaddyService {
	return &caddyService{
		logger:      logging.GetGlobalLogger(),
		client:      &http.Client{},
		baseURL:     "http://172.20.0.4:2019",
		connections: connections,
		config:      cfg,
	}
}

// ValidateConnection checks if we can connect to Caddy's admin API
func (s *caddyService) ValidateConnection() error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/config/", s.baseURL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Caddy admin API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Caddy admin API returned unexpected status %d: %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully connected to Caddy admin API")
	return nil
}

// ConfigureRoute adds or updates a reverse proxy route in Caddy
func (s *caddyService) ConfigureRoute(domain string, targetIP string, targetPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If it's a subdomain of the base domain, we use the wildcard policy.
	// Otherwise, we let On-Demand TLS handle it (configured in LoadConfig).
	if isSubdomain(domain, s.config.BaseDomain) {
		if err := s.ensureWildcardPolicy(s.config.BaseDomain); err != nil {
			return fmt.Errorf("failed to ensure wildcard policy: %w", err)
		}
	}

	// Create route configuration
	config := map[string]interface{}{
		"@id": domain,
		"handle": []map[string]interface{}{
			{
				"handler": "headers",
				"response": map[string]interface{}{
					"set": map[string]interface{}{
						"Strict-Transport-Security": []string{"max-age=31536000; includeSubDomains; preload"},
						"X-Content-Type-Options":    []string{"nosniff"},
						"X-Frame-Options":           []string{"DENY"},
						"X-XSS-Protection":          []string{"1; mode=block"},
						"Content-Security-Policy":   []string{"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'"},
						"Referrer-Policy":           []string{"strict-origin-when-cross-origin"},
						"Permissions-Policy":        []string{"geolocation=(), microphone=(), camera=()"},
					},
					"delete": []string{"Server", "X-Powered-By"},
				},
			},
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{
						"dial": "api:8081", // Forward to our tunnel server
					},
				},
				"transport": map[string]interface{}{
					"protocol":                "http",
					"read_timeout":            "30m", // 30 minutes for large uploads
					"write_timeout":           "30m", // 30 minutes for large downloads
					"dial_timeout":            "10s", // 10 seconds to establish connection
					"response_header_timeout": "30m", // 30 minutes to wait for response headers
				},
				"headers": map[string]interface{}{
					"request": map[string]interface{}{
						"set": map[string]interface{}{
							"Host":              []string{domain}, // Preserve the original host
							"X-Real-IP":         []string{"{http.request.remote.host}"},
							"X-Forwarded-For":   []string{"{http.request.remote.host}"},
							"X-Forwarded-Proto": []string{"{http.request.scheme}"},
						},
					},
				},
			},
		},
		"match": []map[string]interface{}{
			{
				"host": []string{domain},
			},
		},
		"terminal": true,
	}

	// Convert config to JSON
	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Send config to Caddy
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", s.baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonConfig))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to configure route (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully configured route for domain: %s (tunnel proxy)", domain)
	return nil
}

// isSubdomain checks if a domain is a subdomain of a base domain
func isSubdomain(domain, base string) bool {
	if domain == base {
		return true
	}
	return len(domain) > len(base) && domain[len(domain)-len(base)-1] == '.' && domain[len(domain)-len(base):] == base
}

// ensureWildcardPolicy ensures that the wildcard policy for a base domain exists
func (s *caddyService) ensureWildcardPolicy(baseDomain string) error {
	wildcard := "*." + baseDomain

	// Get current config
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/config/", s.baseURL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}
	defer resp.Body.Close()

	var currentConfig map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&currentConfig); err != nil {
		return fmt.Errorf("failed to decode current config: %w", err)
	}

	apps, _ := currentConfig["apps"].(map[string]interface{})
	tls, _ := apps["tls"].(map[string]interface{})
	automation, _ := tls["automation"].(map[string]interface{})
	policies, ok := automation["policies"].([]interface{})
	if !ok {
		policies = []interface{}{}
	}

	// Check if wildcard policy already exists
	found := false
	for _, p := range policies {
		policy := p.(map[string]interface{})
		subjects, ok := policy["subjects"].([]interface{})
		if !ok {
			continue
		}
		for _, s := range subjects {
			if s == wildcard {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		// Create a wildcard policy with Cloudflare DNS challenge
		issuer := map[string]interface{}{
			"module": "acme",
		}

		// Inject Cloudflare token if configured
		if s.config.CloudflareToken != "" {
			issuer["challenges"] = map[string]interface{}{
				"dns": map[string]interface{}{
					"provider": map[string]interface{}{
						"name":      "cloudflare",
						"api_token": s.config.CloudflareToken,
					},
				},
			}
		} else {
			s.logger.Warn("Cloudflare API token not configured. Wildcard certificate for %s may fail.", baseDomain)
		}

		newPolicy := map[string]interface{}{
			"subjects": []string{wildcard, baseDomain},
			"issuers":  []interface{}{issuer},
		}
		automation["policies"] = append([]interface{}{newPolicy}, policies...)

		jsonConfig, err := json.Marshal(currentConfig)
		if err != nil {
			return err
		}

		req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/load", s.baseURL), bytes.NewBuffer(jsonConfig))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err = s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update wildcard policy (status %d): %s", resp.StatusCode, string(body))
		}
		s.logger.Info("Successfully added wildcard policy for %s", baseDomain)
	}

	return nil
}

// ensureTLSPolicy is now deprecated and kept for backward compatibility if needed,
// but renamed/internally bypassed to avoid linear subject growth.
func (s *caddyService) ensureTLSPolicy(domain string) error {
	// Custom domains are now handled by On-Demand TLS (see LoadConfig).
	// Subdomains of the base domain are handled by ensureWildcardPolicy.
	if isSubdomain(domain, s.config.BaseDomain) {
		return s.ensureWildcardPolicy(s.config.BaseDomain)
	}
	return nil
}

// RemoveRoute removes a route from Caddy configuration
func (s *caddyService) RemoveRoute(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, get all routes to find the numeric index of our domain's route
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", s.baseURL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get routes: %w", err)
	}
	defer resp.Body.Close()

	// Decode routes array
	var routes []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return fmt.Errorf("failed to decode routes: %w", err)
	}

	// Find the index of the route with matching domain
	routeIndex := -1
	for i, route := range routes {
		if match, ok := route["match"].([]interface{}); ok && len(match) > 0 {
			if matchMap, ok := match[0].(map[string]interface{}); ok {
				if hosts, ok := matchMap["host"].([]interface{}); ok && len(hosts) > 0 {
					if host, ok := hosts[0].(string); ok && host == domain {
						routeIndex = i
						break
					}
				}
			}
		}
	}

	// If route not found, it might already be removed
	if routeIndex == -1 {
		s.logger.Info("Route for domain %s not found (may already be removed)", domain)
		return nil // Not an error - route is already gone
	}

	// Send DELETE request to Caddy using numeric index
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d", s.baseURL, routeIndex)
	req, err = http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err = s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove route (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully removed route for domain: %s (index %d)", domain, routeIndex)
	return nil
}

// LoadConfig loads the initial Caddy configuration
func (s *caddyService) LoadConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First get the current config
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/config/", s.baseURL), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}
	defer resp.Body.Close()

	var currentConfig map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&currentConfig); err != nil {
		return fmt.Errorf("failed to decode current config: %w", err)
	}

	// Ensure required configuration exists
	if apps, ok := currentConfig["apps"].(map[string]interface{}); ok {
		// 1. Configure On-Demand TLS 'ask' endpoint
		tls, ok := apps["tls"].(map[string]interface{})
		if !ok {
			tls = make(map[string]interface{})
			apps["tls"] = tls
		}
		automation, ok := tls["automation"].(map[string]interface{})
		if !ok {
			automation = make(map[string]interface{})
			tls["automation"] = automation
		}
		onDemand, ok := automation["on_demand"].(map[string]interface{})
		if !ok {
			onDemand = make(map[string]interface{})
			automation["on_demand"] = onDemand
		}
		// Point to our internal API endpoint
		onDemand["ask"] = "http://api:8080/api/v1/caddy/check-domain"

		// 2. Configure HTTP server settings
		if http, ok := apps["http"].(map[string]interface{}); ok {
			if servers, ok := http["servers"].(map[string]interface{}); ok {
				if srv0, ok := servers["srv0"].(map[string]interface{}); ok {
					// Ensure automatic HTTPS is enabled
					srv0["automatic_https"] = map[string]interface{}{
						"disable": false,
					}
					// Ensure proper listening addresses
					srv0["listen"] = []string{":80", ":443"}
				}
			}
		}
	}

	// Convert config back to JSON
	jsonConfig, err := json.Marshal(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Send updated config back to Caddy
	req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/load", s.baseURL), bytes.NewBuffer(jsonConfig))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err = s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to load config (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully loaded and updated Caddy configuration")
	return nil
}

// ConfigureTunnelRoute adds or updates a tunnel route in Caddy
func (s *caddyService) ConfigureTunnelRoute(domain string, targetIP string, targetPort int) error {
	return s.ConfigureRoute(domain, targetIP, targetPort)
}

// RemoveTunnelRoute removes a tunnel route from Caddy configuration
func (s *caddyService) RemoveTunnelRoute(domain string) error {
	return s.RemoveRoute(domain)
}
