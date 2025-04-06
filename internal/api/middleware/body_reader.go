package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"giraffecloud/internal/api/dto/common"

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
		c.Set("body_validation", option)
		c.Next()
	}
}

// PreserveRequestBody middleware reads and preserves the request body so it can be read multiple times
// It does not validate or reject empty bodies by default
func PreserveRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.Request.URL.Path

		// Only process request methods that may have a body
		if method == http.MethodPost ||
		   method == http.MethodPut ||
		   method == http.MethodPatch {

			log.Printf("Processing %s request to %s", method, path)

			// Read raw body
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err != nil && err != io.EOF {
				log.Printf("Error reading request body for %s %s: %v", method, path, err)
				response := common.NewErrorResponse(common.ErrCodeBadRequest, "Error reading request body", err)
				c.JSON(http.StatusBadRequest, response)
				c.Abort()
				return
			}

			bodySize := len(bodyBytes)
			log.Printf("Request body size for %s %s: %d bytes", method, path, bodySize)

			// Store the raw body in the context for further use
			c.Set("raw_body", bodyBytes)

			// Restore the body for the next middlewares/handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		c.Next()
	}
}