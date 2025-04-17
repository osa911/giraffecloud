package middleware

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CSRFMiddleware checks CSRF token for unsafe methods
func CSRFMiddleware(csrfService service.CSRFService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		csrfCookie, err := c.Cookie(constants.CookieCSRF)
		csrfHeader := c.GetHeader(constants.HeaderCSRF)
		if err != nil || !csrfService.ValidateToken(csrfCookie, csrfHeader) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token invalid or missing"})
			return
		}
		c.Next()
	}
}
