package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"giraffecloud/internal/logging"
	"io"
	"net"
	"net/http"
	"sync"
)

// CaddyService defines the interface for Caddy operations
type CaddyService interface {
	ConfigureRoute(domain string, targetIP string, targetPort int) error
	RemoveRoute(domain string) error
	ValidateConnection() error
	LoadConfig() error
}

type caddyService struct {
	logger   *logging.Logger
	client   *http.Client
	mu       sync.RWMutex
}

// NewCaddyService creates a new Caddy service instance
func NewCaddyService() CaddyService {
	// Create a custom HTTP client that uses Unix domain sockets
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/caddy/admin.sock")
			},
		},
	}

	return &caddyService{
		logger: logging.GetGlobalLogger(),
		client: client,
	}
}

// ValidateConnection checks if we can connect to Caddy's admin API
func (s *caddyService) ValidateConnection() error {
	req, err := http.NewRequest(http.MethodGet, "/config/", nil)
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
							"Host":              []string{"{http.request.host}"},
							"X-Real-IP":         []string{"{http.request.remote}"},
							"X-Forwarded-For":   []string{"{http.request.remote}"},
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
	req, err := http.NewRequest(http.MethodPut,
		"/config/apps/http/servers/main/routes/"+domain,
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
	req, err := http.NewRequest(http.MethodDelete,
		"/config/apps/http/servers/main/routes/"+domain,
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