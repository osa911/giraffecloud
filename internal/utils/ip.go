package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GetRealIP extracts the client IP from various headers, respecting reverse proxies
// This function is used consistently across the application to ensure accurate IP tracking
func GetRealIP(c *gin.Context) string {
	// Try X-Real-IP first (set by Caddy)
	ip := c.GetHeader("X-Real-IP")
	if ip != "" {
		return ip
	}

	// Try X-Forwarded-For next (also set by proxies)
	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		// X-Forwarded-For can be a comma-separated list
		// Format: client, proxy1, proxy2, ...
		// We want the first (leftmost) IP which is the client
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			return clientIP
		}
	}

	// Fall back to RemoteAddr from Gin's ClientIP
	return c.ClientIP()
}
