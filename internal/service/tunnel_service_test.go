package service

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/tunnel"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/repository"
	"github.com/osa911/giraffecloud/internal/utils"
)

// Mock TunnelRepository
type mockTunnelRepository struct {
	repository.TunnelRepository
	createFunc      func(ctx context.Context, tunnel *ent.Tunnel) (*ent.Tunnel, error)
	getByUserIDFunc func(ctx context.Context, userID uint32) ([]*ent.Tunnel, error)
	getByIDFunc     func(ctx context.Context, id uint32) (*ent.Tunnel, error)
	updateFunc      func(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error)
}

func (m *mockTunnelRepository) Create(ctx context.Context, tunnel *ent.Tunnel) (*ent.Tunnel, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, tunnel)
	}
	return tunnel, nil
}

func (m *mockTunnelRepository) GetByUserID(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return []*ent.Tunnel{}, nil
}

func (m *mockTunnelRepository) GetByID(ctx context.Context, id uint32) (*ent.Tunnel, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("tunnel not found")
}

func (m *mockTunnelRepository) Update(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, id, updates)
	}
	return nil, nil
}

// Mock CaddyService
type mockCaddyService struct {
	CaddyService
}

func TestCreateTunnel_DNSVerification(t *testing.T) {
	// Initialize logger for testing
	logging.InitLogger(&logging.LogConfig{
		Level:  "info",
		Format: "text",
		File:   "test.log", // Dummy file
	})

	// Set CLIENT_URL for domain generation logic
	os.Setenv("CLIENT_URL", "https://giraffecloud.xyz")
	defer os.Unsetenv("CLIENT_URL")

	// Save original lookupHost and restore after test
	originalLookupHost := lookupHost
	defer func() { lookupHost = originalLookupHost }()

	tests := []struct {
		name              string
		userID            uint32
		domain            string
		mockDNS           []string
		expectedError     bool
		expectedEnabled   bool
		expectedDnsStatus tunnel.DNSPropagationStatus
	}{
		{
			name:              "Valid Custom Domain",
			userID:            123,
			domain:            "valid.custom.com",
			mockDNS:           []string{"1.2.3.4"}, // Matches server IP
			expectedError:     false,
			expectedEnabled:   true,
			expectedDnsStatus: tunnel.DNSPropagationStatusVerified,
		},
		{
			name:              "Invalid Custom Domain",
			userID:            123,
			domain:            "invalid.custom.com",
			mockDNS:           []string{"9.9.9.9"}, // Mismatch
			expectedError:     false,
			expectedEnabled:   false,
			expectedDnsStatus: tunnel.DNSPropagationStatusPendingDNS,
		},
		{
			name:              "DNS Error",
			userID:            123,
			domain:            "error.custom.com",
			mockDNS:           nil, // DNS lookup error
			expectedError:     false,
			expectedEnabled:   false,
			expectedDnsStatus: tunnel.DNSPropagationStatusPendingDNS,
		},
		{
			name:              "Auto-generated Domain (Skip Check)",
			userID:            123,
			domain:            utils.GenerateSubdomainForUser(123),
			mockDNS:           []string{"9.9.9.9"}, // Should be ignored
			expectedError:     false,
			expectedEnabled:   true,
			expectedDnsStatus: tunnel.DNSPropagationStatusVerified,
		},
		{
			name:              "Server IP Not Set (Skip Check)",
			userID:            123,
			domain:            "any.custom.com",
			mockDNS:           []string{"9.9.9.9"},
			expectedError:     false,
			expectedEnabled:   true,
			expectedDnsStatus: tunnel.DNSPropagationStatusVerified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock DNS
			if tt.name == "DNS Error" {
				lookupHost = func(host string) ([]string, error) {
					return nil, fmt.Errorf("dns error")
				}
			} else {
				lookupHost = func(host string) ([]string, error) {
					return tt.mockDNS, nil
				}
			}

			// Setup mock repo
			mockRepo := &mockTunnelRepository{
				createFunc: func(ctx context.Context, tunnel *ent.Tunnel) (*ent.Tunnel, error) {
					return tunnel, nil
				},
				getByUserIDFunc: func(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
					return []*ent.Tunnel{}, nil
				},
			}

			// Setup service with or without ServerIP
			serverIP := "1.2.3.4"
			if tt.name == "Server IP Not Set (Skip Check)" {
				serverIP = ""
			}
			mockCaddy := &mockCaddyService{}
			svc := NewTunnelService(mockRepo, mockCaddy, serverIP)

			// Execute
			tunnel, err := svc.CreateTunnel(context.Background(), tt.userID, tt.domain, 8080)

			// Verify
			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tunnel == nil {
					t.Fatal("expected tunnel, got nil")
				}
				if tunnel.IsEnabled != tt.expectedEnabled {
					t.Errorf("expected enabled=%v, got %v", tt.expectedEnabled, tunnel.IsEnabled)
				}
				if tunnel.DNSPropagationStatus != tt.expectedDnsStatus {
					t.Errorf("expected dns status=%s, got %s", tt.expectedDnsStatus, tunnel.DNSPropagationStatus)
				}
			}
		})
	}
}

func TestUpdateTunnel_DNSVerification(t *testing.T) {
	// Initialize logger for testing
	logging.InitLogger(&logging.LogConfig{
		Level:  "info",
		Format: "text",
		File:   "test_update.log",
	})

	// Save original lookupHost and restore after test
	originalLookupHost := lookupHost
	defer func() { lookupHost = originalLookupHost }()

	tests := []struct {
		name          string
		domain        string
		serverIP      string
		isEnabled     bool // Current state
		newEnabled    bool // New state
		mockDNS       func(host string) ([]string, error)
		expectedError bool
	}{
		{
			name:       "Enable Valid Custom Domain",
			domain:     "valid.example.com",
			serverIP:   "1.2.3.4",
			isEnabled:  false,
			newEnabled: true,
			mockDNS: func(host string) ([]string, error) {
				return []string{"1.2.3.4"}, nil
			},
			expectedError: false,
		},
		{
			name:       "Enable Invalid Custom Domain",
			domain:     "invalid.example.com",
			serverIP:   "1.2.3.4",
			isEnabled:  false,
			newEnabled: true,
			mockDNS: func(host string) ([]string, error) {
				return []string{"5.6.7.8"}, nil
			},
			expectedError: true,
		},
		{
			name:       "Enable with DNS Error",
			domain:     "error.example.com",
			serverIP:   "1.2.3.4",
			isEnabled:  false,
			newEnabled: true,
			mockDNS: func(host string) ([]string, error) {
				return nil, fmt.Errorf("dns error")
			},
			expectedError: true,
		},
		{
			name:       "Disable Tunnel (No Check)",
			domain:     "any.example.com",
			serverIP:   "1.2.3.4",
			isEnabled:  true,
			newEnabled: false,
			mockDNS: func(host string) ([]string, error) {
				return []string{"5.6.7.8"}, nil // Should be ignored
			},
			expectedError: false,
		},
		{
			name:       "Enable Auto-generated Domain (Skip Check)",
			domain:     utils.GenerateSubdomainForUser(123),
			serverIP:   "1.2.3.4",
			isEnabled:  false,
			newEnabled: true,
			mockDNS: func(host string) ([]string, error) {
				return []string{"5.6.7.8"}, nil // Should be ignored
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock DNS
			lookupHost = tt.mockDNS

			// Setup mock repo
			mockRepo := &mockTunnelRepository{
				getByIDFunc: func(ctx context.Context, id uint32) (*ent.Tunnel, error) {
					return &ent.Tunnel{
						ID:         int(id),
						Domain:     tt.domain,
						IsEnabled:  tt.isEnabled,
						TargetPort: 8080,
						UserID:     123,
					}, nil
				},
				updateFunc: func(ctx context.Context, id uint32, updates interface{}) (*ent.Tunnel, error) {
					return &ent.Tunnel{
						ID:        int(id),
						Domain:    tt.domain,
						IsEnabled: tt.newEnabled,
					}, nil
				},
				getByUserIDFunc: func(ctx context.Context, userID uint32) ([]*ent.Tunnel, error) {
					return []*ent.Tunnel{}, nil
				},
			}

			// Setup service
			svc := NewTunnelService(mockRepo, &mockCaddyService{}, tt.serverIP)

			// Execute
			updates := &repository.TunnelUpdate{
				IsEnabled: &tt.newEnabled,
			}
			_, err := svc.UpdateTunnel(context.Background(), 123, 1, updates)

			// Verify
			if (err != nil) != tt.expectedError {
				t.Errorf("UpdateTunnel() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}
