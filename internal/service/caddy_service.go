package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"io"
	"net/http"
	"sync"
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
}

// NewCaddyService creates a new Caddy service instance
func NewCaddyService(connections *tunnel.ConnectionManager) CaddyService {
	return &caddyService{
		logger:      logging.GetGlobalLogger(),
		client:      &http.Client{},
		baseURL:     "http://172.20.0.4:2019",
		connections: connections,
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

	// First, ensure the domain is in the TLS automation policy
	if err := s.ensureTLSPolicy(domain); err != nil {
		return fmt.Errorf("failed to configure TLS policy: %w", err)
	}

	// Create route configuration
	config := map[string]interface{}{
		"@id": domain,
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{
						"dial": "api:8081", // Forward to our API service that handles tunnels
					},
				},
				"transport": map[string]interface{}{
					"protocol": "http",
				},
				"headers": map[string]interface{}{
					"request": map[string]interface{}{
						"set": map[string]interface{}{
							"Host": []string{domain}, // Preserve the original host
							"X-Real-IP": []string{"{http.request.remote.host}"},
							"X-Forwarded-For": []string{"{http.request.remote.host}"},
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

// ensureTLSPolicy ensures that the domain is included in Caddy's TLS automation policy
func (s *caddyService) ensureTLSPolicy(domain string) error {
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

	// Navigate to TLS automation policies
	apps, ok := currentConfig["apps"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid config structure: apps not found")
	}

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

	policies, ok := automation["policies"].([]interface{})
	if !ok || len(policies) == 0 {
		// Create default policy if none exists
		policies = []interface{}{
			map[string]interface{}{
				"subjects": []string{"giraffecloud.xyz", "*.giraffecloud.xyz"},
				"issuers": []interface{}{
					map[string]interface{}{
						"module": "acme",
						"ca":     "https://acme-v02.api.letsencrypt.org/directory",
						"challenges": map[string]interface{}{
							"http": map[string]interface{}{
								"disabled": false,
							},
							"dns": map[string]interface{}{
								"disabled": true,
							},
							"tls-alpn": map[string]interface{}{
								"disabled": true,
							},
						},
					},
				},
			},
		}
		automation["policies"] = policies
	}

	// Check if domain is already in policy
	policy := policies[0].(map[string]interface{})
	subjects, ok := policy["subjects"].([]interface{})
	if !ok {
		subjects = []interface{}{}
	}

	// Convert subjects to strings for comparison
	subjectStrings := make([]string, len(subjects))
	for i, s := range subjects {
		subjectStrings[i] = s.(string)
	}

	// Add domain if not already present
	domainFound := false
	for _, s := range subjectStrings {
		if s == domain {
			domainFound = true
			break
		}
	}

	if !domainFound {
		subjectStrings = append(subjectStrings, domain)
		policy["subjects"] = subjectStrings
	}

	// Update config if domain was added
	if !domainFound {
		jsonConfig, err := json.Marshal(currentConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal updated config: %w", err)
		}

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
			return fmt.Errorf("failed to update TLS policy (status %d): %s", resp.StatusCode, string(body))
		}

		s.logger.Info("Successfully added domain %s to TLS automation policy", domain)
	}

	return nil
}

// RemoveRoute removes a route from Caddy configuration
func (s *caddyService) RemoveRoute(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send DELETE request to Caddy
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/@%s", s.baseURL, domain)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove route (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully removed route for domain: %s", domain)
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