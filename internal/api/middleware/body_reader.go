package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/osa911/giraffecloud/internal/api/constants"

	"github.com/gin-gonic/gin"
)

// BodyValidationOption defines options for request body validation
type BodyValidationOption int

const (
	// RequireBody means the request must have a non-empty body
	RequireBody BodyValidationOption = iota
	// AllowEmptyBody means the request can have an empty body
	AllowEmptyBody
)

// SetBodyValidation sets the body validation option for a route
func SetBodyValidation(option BodyValidationOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(constants.ContextKeyBodyValidation, option)
		c.Next()
	}
}

// BodyReaderOption defines options for body reader middleware
type BodyReaderOption struct {
	MaxBodySize int64
}

// PreserveRequestBody middleware reads the request body once and restores it
// This allows validators and controllers to both read the body
func PreserveRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the configured options or use defaults
		option := BodyReaderOption{
			MaxBodySize: 10 * 1024 * 1024, // 10 MB default
		}

		// Store options in context for potential use later
		c.Set(constants.ContextKeyBodyValidation, option)

		// Only process POST, PUT, PATCH and DELETE requests with request body
		if c.Request.Body == nil || (c.Request.Method != "POST" && c.Request.Method != "PUT" && c.Request.Method != "PATCH" && c.Request.Method != "DELETE") {
			c.Next()
			return
		}

		var bodyBytes []byte
		bodyBytes, err := io.ReadAll(c.Request.Body)

		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		// Check for content length or malformed requests
		if (c.Request.ContentLength == 0 && len(bodyBytes) > 0) || (c.Request.ContentLength > 0 && int64(len(bodyBytes)) != c.Request.ContentLength) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Check max body size (configurable)
		if int64(len(bodyBytes)) > option.MaxBodySize {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}

		// Restore the body for subsequent middleware
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Store body in context for potential use later
		c.Set(constants.ContextKeyRawBody, bodyBytes)

		c.Next()
	}
}
