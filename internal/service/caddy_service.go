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
		"handle": []map[string]interface{}{
			{
				"handler": "reverse_proxy",
				"flush_interval": "0s",
				"upstreams": []map[string]interface{}{
					{
						"dial": "api:8081",
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

	// After marshaling config to JSON in ConfigureRoute:
	s.logger.Info("[DEBUG] ConfigureRoute: JSON body: %s", string(jsonConfig))

	// Before making the HTTP request in ConfigureRoute:
	s.logger.Info("[DEBUG] ConfigureRoute: domain=%q, targetIP=%q, targetPort=%d", domain, targetIP, targetPort)
	s.logger.Info("[DEBUG] ConfigureRoute: PATCHing routes array at /config/apps/http/servers/srv0/routes")
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", s.baseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonConfig))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-HTTP-Method-Override", "PATCH")
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

	s.logger.Info("[DEBUG] RemoveRoute: domain=%q", domain)

	// 1. Get current routes array
	getURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", s.baseURL)
	resp, err := s.client.Get(getURL)
	if err != nil {
		return fmt.Errorf("failed to GET routes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to GET routes (status %d): %s", resp.StatusCode, string(body))
	}
	var routes []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return fmt.Errorf("failed to decode routes: %w", err)
	}

	// 2. Find the index of the route with @id == domain
	index := -1
	for i, route := range routes {
		if id, ok := route["@id"].(string); ok && id == domain {
			index = i
			break
		}
	}
	if index == -1 {
		s.logger.Info("[DEBUG] RemoveRoute: route with @id=%q not found, nothing to remove", domain)
		return nil
	}

	// 3. PATCH to remove the route at that index
	patchBody := []map[string]interface{}{{
		"op":   "remove",
		"path": fmt.Sprintf("/%d", index),
	}}
	jsonPatch, err := json.Marshal(patchBody)
	if err != nil {
		return fmt.Errorf("failed to marshal patch body: %w", err)
	}
	s.logger.Info("[DEBUG] RemoveRoute: PATCH body: %s", string(jsonPatch))
	patchURL := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", s.baseURL)
	req, err := http.NewRequest("PATCH", patchURL, bytes.NewBuffer(jsonPatch))
	if err != nil {
		return fmt.Errorf("failed to create PATCH request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PATCH request: %w", err)
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

	// After marshaling config to JSON in ConfigureTunnelRoute:
	s.logger.Info("[DEBUG] ConfigureTunnelRoute: JSON body: %s", string(jsonConfig))

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