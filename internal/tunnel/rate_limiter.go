package tunnel

import (
	"sync"
	"time"
)

// RateLimiterConfig holds rate limiting configuration
type RateLimiterConfig struct {
	MaxRequestsPerSecond  int           // Maximum requests per second
	MaxConcurrentRequests int           // Maximum concurrent requests
	BurstSize            int           // Number of requests that can exceed rate
	RetryAfter           time.Duration // How long to wait before retrying
	BlockDuration        time.Duration // How long to block after exceeding limits
}

// DefaultRateLimiterConfig returns default rate limiting settings
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		MaxRequestsPerSecond:  100,
		MaxConcurrentRequests: 50,
		BurstSize:            20,
		RetryAfter:           time.Second * 1,
		BlockDuration:        time.Minute * 1,
	}
}

// RateLimiter implements rate limiting for tunnel connections
type RateLimiter struct {
	config *RateLimiterConfig
	mu     sync.RWMutex

	// Per-connection tracking
	connections map[string]*connectionState

	// Global tracking
	globalRequests     int32
	globalLastCleanup  time.Time
	globalRequestCount int32
}

type connectionState struct {
	requestCount    int32
	lastRequest     time.Time
	blockedUntil    time.Time
	currentRequests int32
	bucketTokens    int
	lastRefill      time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}

	rl := &RateLimiter{
		config:      config,
		connections: make(map[string]*connectionState),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(connID string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create connection state
	state, exists := rl.connections[connID]
	if !exists {
		state = &connectionState{
			lastRefill: now,
			bucketTokens: rl.config.BurstSize,
		}
		rl.connections[connID] = state
	}

	// Check if connection is blocked
	if !state.blockedUntil.IsZero() && now.Before(state.blockedUntil) {
		return false, state.blockedUntil.Sub(now)
	}

	// Refill token bucket
	timeSinceRefill := now.Sub(state.lastRefill)
	tokensToAdd := int(timeSinceRefill.Seconds() * float64(rl.config.MaxRequestsPerSecond))
	state.bucketTokens = min(rl.config.BurstSize, state.bucketTokens + tokensToAdd)
	state.lastRefill = now

	// Check rate limits
	if state.bucketTokens <= 0 {
		// Rate exceeded, block the connection
		state.blockedUntil = now.Add(rl.config.BlockDuration)
		return false, rl.config.BlockDuration
	}

	// Check concurrent requests
	if state.currentRequests >= int32(rl.config.MaxConcurrentRequests) {
		return false, rl.config.RetryAfter
	}

	// Update counters
	state.bucketTokens--
	state.currentRequests++
	state.requestCount++
	state.lastRequest = now

	return true, 0
}

// Release marks a request as completed
func (rl *RateLimiter) Release(connID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if state, exists := rl.connections[connID]; exists {
		if state.currentRequests > 0 {
			state.currentRequests--
		}
	}
}

// cleanup periodically removes stale connection states
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for id, state := range rl.connections {
			// Remove if inactive for more than 5 minutes and not blocked
			if now.Sub(state.lastRequest) > 5*time.Minute &&
			   (state.blockedUntil.IsZero() || now.After(state.blockedUntil)) {
				delete(rl.connections, id)
			}
		}

		rl.mu.Unlock()
	}
}

// GetStats returns current rate limiting statistics
func (rl *RateLimiter) GetStats(connID string) map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	stats := make(map[string]interface{})
	if state, exists := rl.connections[connID]; exists {
		stats["request_count"] = state.requestCount
		stats["current_requests"] = state.currentRequests
		stats["tokens_available"] = state.bucketTokens
		stats["is_blocked"] = !state.blockedUntil.IsZero() && time.Now().Before(state.blockedUntil)
		if stats["is_blocked"].(bool) {
			stats["blocked_until"] = state.blockedUntil
		}
	}

	return stats
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}