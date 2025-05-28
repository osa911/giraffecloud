package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// StickyConfig holds sticky session configuration
type StickyConfig struct {
	Enabled       bool          // Whether sticky sessions are enabled
	CookieName    string        // Name of the cookie to use
	CookieTimeout time.Duration // How long the sticky cookie is valid
	HashSecret    string        // Secret for hashing client info
}

// DefaultStickyConfig returns default sticky session settings
func DefaultStickyConfig() *StickyConfig {
	return &StickyConfig{
		Enabled:       true,
		CookieName:    "TUNNELID",
		CookieTimeout: time.Hour * 24,
		HashSecret:    "change-me-in-production", // Should be configured in production
	}
}

// StickyManager manages sticky sessions for tunnel connections
type StickyManager struct {
	config *StickyConfig
	mu     sync.RWMutex

	// Maps client IDs to tunnel connections
	sessions map[string]string // clientID -> tunnelID

	// Maps tunnel IDs to last access time
	lastAccess map[string]time.Time
}

// NewStickyManager creates a new sticky session manager
func NewStickyManager(config *StickyConfig) *StickyManager {
	if config == nil {
		config = DefaultStickyConfig()
	}

	sm := &StickyManager{
		config:     config,
		sessions:   make(map[string]string),
		lastAccess: make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go sm.cleanup()

	return sm
}

// GenerateClientID generates a unique client ID based on client information
func (sm *StickyManager) GenerateClientID(clientIP, userAgent string) string {
	// Combine client information with secret
	data := clientIP + userAgent + sm.config.HashSecret

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetTunnel gets the sticky tunnel ID for a client
func (sm *StickyManager) GetTunnel(clientID string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tunnelID, exists := sm.sessions[clientID]
	if exists {
		// Update last access time
		sm.lastAccess[tunnelID] = time.Now()
	}
	return tunnelID, exists
}

// SetTunnel sets the sticky tunnel ID for a client
func (sm *StickyManager) SetTunnel(clientID, tunnelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[clientID] = tunnelID
	sm.lastAccess[tunnelID] = time.Now()
}

// RemoveTunnel removes a sticky session
func (sm *StickyManager) RemoveTunnel(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if tunnelID, exists := sm.sessions[clientID]; exists {
		delete(sm.sessions, clientID)
		delete(sm.lastAccess, tunnelID)
	}
}

// cleanup periodically removes expired sticky sessions
func (sm *StickyManager) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()

		// Find expired sessions
		var expired []string
		for clientID, tunnelID := range sm.sessions {
			lastAccess, exists := sm.lastAccess[tunnelID]
			if !exists || now.Sub(lastAccess) > sm.config.CookieTimeout {
				expired = append(expired, clientID)
			}
		}

		// Remove expired sessions
		for _, clientID := range expired {
			tunnelID := sm.sessions[clientID]
			delete(sm.sessions, clientID)
			delete(sm.lastAccess, tunnelID)
		}

		sm.mu.Unlock()
	}
}

// GetStats returns sticky session statistics
func (sm *StickyManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_sessions"] = len(sm.sessions)
	stats["enabled"] = sm.config.Enabled

	// Calculate session age statistics
	var maxAge time.Duration
	var totalAge time.Duration
	now := time.Now()

	for _, lastAccess := range sm.lastAccess {
		age := now.Sub(lastAccess)
		totalAge += age
		if age > maxAge {
			maxAge = age
		}
	}

	if len(sm.lastAccess) > 0 {
		stats["avg_session_age"] = totalAge / time.Duration(len(sm.lastAccess))
		stats["max_session_age"] = maxAge
	}

	return stats
}