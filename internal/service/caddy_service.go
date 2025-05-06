package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/caddy"
	"giraffecloud/internal/logging"
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
	logger   *logging.Logger
	client   *http.Client
	baseURL  string
	mu       sync.RWMutex
}

// NewCaddyService creates a new Caddy service instance
func NewCaddyService() CaddyService {
	return &caddyService{
		logger:   logging.GetGlobalLogger(),
		client:   &http.Client{},
		baseURL:  "http://172.20.0.4:2019",
	}
}

// ValidateConnection checks if we can connect to Caddy's admin API
func (s *caddyService) ValidateConnection() error {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", s.baseURL, caddy.DefaultAdminEndpoint), nil)
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

	// Create route configuration
	config := map[string]interface{}{
		"@id": domain,
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]interface{}{
					{
						"dial": fmt.Sprintf("%s:%d", targetIP, targetPort),
					},
				},
				"headers": map[string]interface{}{
					"request": map[string]interface{}{
						"set": map[string]interface{}{
							"Host":      []string{"{http.request.host}"},
							"X-Real-IP": []string{"{http.request.remote.host}"},
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

	// Before making the HTTP request in ConfigureRoute:
	s.logger.Info("[DEBUG] ConfigureRoute: domain=%q, targetIP=%q, targetPort=%d", domain, targetIP, targetPort)
	url := fmt.Sprintf("%s/config/id/%s", s.baseURL, domain)
	s.logger.Info("[DEBUG] ConfigureRoute: full URL: %s", url)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonConfig))
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

	s.logger.Info("Successfully configured route for domain: %s -> %s:%d", domain, targetIP, targetPort)
	return nil
}

// RemoveRoute removes a route from Caddy configuration
func (s *caddyService) RemoveRoute(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Before making the HTTP request in RemoveRoute:
	s.logger.Info("[DEBUG] RemoveRoute: domain=%q", domain)
	url := fmt.Sprintf("%s/config/id/%s", s.baseURL, domain)
	s.logger.Info("[DEBUG] RemoveRoute: full URL: %s", url)
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

	// Just validate the connection to Caddy
	return s.ValidateConnection()
}

// ConfigureTunnelRoute adds or updates a tunnel route in Caddy
func (s *caddyService) ConfigureTunnelRoute(domain string, targetIP string, targetPort int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create route configuration for tunnel server
	config := map[string]interface{}{
		"@id": domain,
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"transport": map[string]interface{}{
					"protocol": "http",
					"tls":      map[string]interface{}{},
				},
				"upstreams": []map[string]interface{}{
					{
						"dial": fmt.Sprintf("%s:%d", targetIP, targetPort),
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
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/%sapps/http/servers/tunnel/routes/%s", s.baseURL, caddy.DefaultAdminEndpoint, domain),
		bytes.NewBuffer(jsonConfig))
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
		return fmt.Errorf("failed to configure tunnel route (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully configured tunnel route for domain: %s -> %s:%d", domain, targetIP, targetPort)
	return nil
}

// RemoveTunnelRoute removes a tunnel route from Caddy configuration
func (s *caddyService) RemoveTunnelRoute(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send DELETE request to Caddy
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/%sapps/http/servers/tunnel/routes/%s", s.baseURL, caddy.DefaultAdminEndpoint, domain),
		nil)
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
		return fmt.Errorf("failed to remove tunnel route (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully removed tunnel route for domain: %s", domain)
	return nil
}