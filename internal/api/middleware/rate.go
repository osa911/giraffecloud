package middleware

import (
	"strconv"
	"sync"
	"time"

	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimitConfig defines configuration for the rate limiter
type RateLimitConfig struct {
	// Requests per second
	RPS int
	// Burst size (number of requests that can be made in a single burst)
	Burst int
}

// IPRateLimiter stores rate limiters for different IP addresses
type IPRateLimiter struct {
	ips      map[string]*rate.Limiter
	mu       *sync.RWMutex
	rps      rate.Limit
	burst    int
	cleanupInterval time.Duration
	lastCleanup time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter
func NewIPRateLimiter(rps int, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		ips:      make(map[string]*rate.Limiter),
		mu:       &sync.RWMutex{},
		rps:      rate.Limit(rps),
		burst:    burst,
		cleanupInterval: 10 * time.Minute,
		lastCleanup: time.Now(),
	}
}

// GetLimiter returns the rate limiter for the given IP address
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		// Double check after acquiring write lock
		limiter, exists = i.ips[ip]
		if !exists {
			limiter = rate.NewLimiter(i.rps, i.burst)
			i.ips[ip] = limiter
		}
		i.mu.Unlock()
	}

	// Occasionally clean up old limiters to prevent memory leak
	if time.Since(i.lastCleanup) > i.cleanupInterval {
		go i.cleanup()
	}

	return limiter
}

// cleanup removes limiters that haven't been used recently
func (i *IPRateLimiter) cleanup() {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.lastCleanup = time.Now()
	// Implementation note: This is a simple cleanup that could be enhanced
	// to track last access time for each limiter
}

// RateLimitMiddleware creates a new rate limiting middleware with the given configuration
func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	// Create an IP-based limiter
	ipLimiter := NewIPRateLimiter(config.RPS, config.Burst)

	return func(c *gin.Context) {
		// Get the real client IP, respecting reverse proxy headers
		clientIP := utils.GetRealIP(c)

		// Get the limiter for this IP
		limiter := ipLimiter.GetLimiter(clientIP)

		// Check if we can make a request
		if !limiter.Allow() {
			// If not, return 429 Too Many Requests
			utils.HandleAPIError(c, nil, common.ErrCodeTooManyRequests, "Rate limit exceeded. Please try again later.")
			c.Abort()
			return
		}

		// Get the current limiter state for headers
		currentTokens := limiter.Tokens()

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.RPS))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(int(currentTokens)))

		// Calculate time until next token is available
		waitTime := limiter.Reserve().Delay()
		resetTime := time.Now().Add(waitTime)
		c.Header("X-RateLimit-Reset", resetTime.Format(time.RFC1123))

		c.Next()
	}
}