package tunnel

import (
	"context"
	"sync"
	"time"

	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/logging"
)

// TunnelStatusCache provides fast in-memory lookup of tunnel active status
// with periodic refresh to avoid per-request database queries
type TunnelStatusCache struct {
	// In-memory cache: domain -> isActive
	cache   map[string]bool
	cacheMu sync.RWMutex

	// Dependencies
	tunnelService interfaces.TunnelService
	logger        *logging.Logger

	// Refresh interval
	refreshInterval time.Duration

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewTunnelStatusCache creates a new tunnel status cache
func NewTunnelStatusCache(tunnelService interfaces.TunnelService, refreshInterval time.Duration) *TunnelStatusCache {
	if refreshInterval == 0 {
		refreshInterval = 5 * time.Second // Default: refresh every 5 seconds
	}

	return &TunnelStatusCache{
		cache:           make(map[string]bool),
		tunnelService:   tunnelService,
		logger:          logging.GetGlobalLogger(),
		refreshInterval: refreshInterval,
		stopCh:          make(chan struct{}),
	}
}

// Start begins the background refresh goroutine
func (c *TunnelStatusCache) Start() {
	c.wg.Add(1)
	go c.refreshLoop()
	c.logger.Info("Tunnel status cache started (refresh interval: %v)", c.refreshInterval)
}

// Stop gracefully stops the cache refresh
func (c *TunnelStatusCache) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	c.logger.Info("Tunnel status cache stopped")
}

// IsActive returns whether a tunnel is active (fast in-memory lookup)
// Returns false if tunnel not found in cache
func (c *TunnelStatusCache) IsActive(domain string) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return c.cache[domain]
}

// Invalidate forces a refresh of the cache for a specific domain
// Useful when you know a status changed (e.g., after update API call)
func (c *TunnelStatusCache) Invalidate(domain string) {
	go c.refreshSingle(domain)
}

// InvalidateAll forces a full cache refresh
func (c *TunnelStatusCache) InvalidateAll() {
	go c.refreshAll()
}

// refreshLoop periodically refreshes the entire cache
func (c *TunnelStatusCache) refreshLoop() {
	defer c.wg.Done()

	// Initial refresh
	c.refreshAll()

	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.refreshAll()
		case <-c.stopCh:
			return
		}
	}
}

// refreshAll updates the cache with all active tunnels
func (c *TunnelStatusCache) refreshAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tunnels, err := c.tunnelService.GetActive(ctx)
	if err != nil {
		c.logger.Error("Failed to refresh tunnel status cache: %v", err)
		return
	}

	// Build new cache
	newCache := make(map[string]bool, len(tunnels))
	for _, tunnel := range tunnels {
		newCache[tunnel.Domain] = tunnel.IsActive
	}

	// Atomic swap
	c.cacheMu.Lock()
	c.cache = newCache
	c.cacheMu.Unlock()
}

// refreshSingle updates the cache for a specific domain
func (c *TunnelStatusCache) refreshSingle(domain string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tunnel, err := c.tunnelService.GetByDomain(ctx, domain)
	if err != nil {
		c.logger.Warn("Failed to refresh tunnel status for %s: %v", domain, err)
		// Remove from cache on error
		c.cacheMu.Lock()
		delete(c.cache, domain)
		c.cacheMu.Unlock()
		return
	}

	c.cacheMu.Lock()
	c.cache[domain] = tunnel.IsActive
	c.cacheMu.Unlock()

	c.logger.Debug("Tunnel status cache updated for %s: active=%v", domain, tunnel.IsActive)
}
