package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/osa911/giraffecloud/internal/config"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/db/ent/tunnel"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/utils"
)

// DNSMonitor handles periodic DNS verification for disabled tunnels
type DNSMonitor struct {
	client        *ent.Client
	tunnelService service.CaddyService // We might need this to configure routes when enabling
	serverIP      string
	done          chan struct{}
	wg            sync.WaitGroup
}

// NewDNSMonitor creates a new DNS monitor task
func NewDNSMonitor(client *ent.Client, caddyService service.CaddyService, cfg *config.Config) *DNSMonitor {
	return &DNSMonitor{
		client:        client,
		tunnelService: caddyService,
		serverIP:      cfg.ServerIP,
		done:          make(chan struct{}),
	}
}

// Start begins the DNS monitor task in the background
func (dm *DNSMonitor) Start() {
	if dm.serverIP == "" {
		logging.GetGlobalLogger().Warn("DNSMonitor: SERVER_IP not set, skipping background DNS verification")
		return
	}
	dm.wg.Add(1)
	go dm.runPeriodically()
}

// Stop gracefully stops the DNS monitor task
func (dm *DNSMonitor) Stop() {
	if dm.serverIP == "" {
		return
	}
	close(dm.done)
	dm.wg.Wait()
}

// runPeriodically runs the verification task at regular intervals
func (dm *DNSMonitor) runPeriodically() {
	defer dm.wg.Done()
	logger := logging.GetGlobalLogger()

	logger.Info("Starting DNS monitor task")

	// Run every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := dm.verifyTunnels(); err != nil {
				logger.Error("Periodic DNS verification failed: %v", err)
			}
		case <-dm.done:
			logger.Info("DNS monitor task stopped")
			return
		}
	}
}

// verifyTunnels performs the actual DNS verification
func (dm *DNSMonitor) verifyTunnels() error {
	ctx := context.Background()
	logger := logging.GetGlobalLogger()

	// Find all tunnels waiting for DNS verification
	tunnels, err := dm.client.Tunnel.Query().
		Where(tunnel.DNSPropagationStatusEQ(tunnel.DNSPropagationStatusPendingDNS)).
		Where(tunnel.IsEnabledEQ(false)).
		All(ctx)

	if err != nil {
		return fmt.Errorf("failed to query disabled tunnels: %w", err)
	}

	for _, t := range tunnels {
		// Perform DNS lookup
		ips, err := utils.LookupHostGlobal(t.Domain)
		if err != nil {
			// DNS lookup failed, skip
			continue
		}

		// Check if any IP matches server IP
		matched := false
		for _, ip := range ips {
			if ip == dm.serverIP {
				matched = true
				break
			}
		}

		if matched {
			logger.Info("DNS verification successful for tunnel %d (%s). Enabling tunnel.", t.ID, t.Domain)

			// Update tunnel status to verified and enable it
			verifiedStatus := tunnel.DNSPropagationStatusVerified
			enabled := true

			// Use repository update struct if available, or direct client update
			// Since we don't have easy access to repo here, we use client directly
			_, err := dm.client.Tunnel.UpdateOneID(t.ID).
				SetIsEnabled(enabled).
				SetDNSPropagationStatus(verifiedStatus).
				Save(ctx)

			if err != nil {
				logger.Error("Failed to enable tunnel %d: %v", t.ID, err)
				continue
			}

			// Configure Caddy route if client IP is present
			if t.ClientIP != "" && dm.tunnelService != nil {
				if err := dm.tunnelService.ConfigureRoute(t.Domain, t.ClientIP, t.TargetPort); err != nil {
					logger.Error("Failed to configure Caddy route for tunnel %d: %v", t.ID, err)
				} else {
					logger.Info("Successfully configured Caddy route for enabled tunnel: %s", t.Domain)
				}
			}
		}
	}

	return nil
}
