package middleware

import (
	"giraffecloud/internal/logging"
	"giraffecloud/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger returns a middleware that logs HTTP requests using the provided logger
func RequestLogger(logger *logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Get request details
		method := c.Request.Method
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}
		clientIP := utils.GetRealIP(c)
		statusCode := c.Writer.Status()
		bytes := c.Writer.Size()
		latency := time.Since(start)

		// Log the request
		logger.LogHTTPRequest(method, path, clientIP, statusCode, bytes, latency.String())

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.LogHTTPError(method, path, clientIP, statusCode, "Request error", err)
			}
		}
	}
}