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
	logger       *logging.Logger
	client       *http.Client
	baseURL      string
	mu           sync.RWMutex
	tunnelServer *tunnel.TunnelServer
}

// NewCaddyService creates a new Caddy service instance
func NewCaddyService(tunnelServer *tunnel.TunnelServer) CaddyService {
	return &caddyService{
		logger:   logging.GetGlobalLogger(),
		client:   &http.Client{},
		baseURL:  "http://172.20.0.4:2019",
		tunnelServer: tunnelServer,
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
				"transport": map[string]interface{}{
					"protocol": "http",
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

	s.logger.Info("Successfully configured route for domain: %s -> %s:%d", domain, targetIP, targetPort)
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
	// Initial configuration for Caddy
	config := map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"srv0": map[string]interface{}{
						"listen": []string{":443"},
						"routes": []interface{}{},
						"automatic_https": map[string]interface{}{
							"disable": false, // Enable automatic HTTPS
						},
					},
				},
			},
		},
	}

	// Convert config to JSON
	jsonConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Send config to Caddy
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/load", s.baseURL), bytes.NewBuffer(jsonConfig))
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
		return fmt.Errorf("failed to load config (status %d): %s", resp.StatusCode, string(body))
	}

	s.logger.Info("Successfully loaded initial Caddy configuration")
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