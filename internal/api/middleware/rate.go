package middleware

import (
	"net/http"
	"strconv"
	"time"

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

// RateLimitMiddleware creates a new rate limiting middleware with the given configuration
func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	// Create a new limiter with the given rate and burst
	limiter := rate.NewLimiter(rate.Limit(config.RPS), config.Burst)

	return func(c *gin.Context) {
		// Check if we can make a request
		if !limiter.Allow() {
			// If not, return 429 Too Many Requests
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
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