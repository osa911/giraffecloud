package middleware

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CSRFMiddleware checks CSRF token for unsafe methods when using cookie-based auth
func CSRFMiddleware(csrfService service.CSRFService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for non-browser requests (CLI/GitHub Actions)
		if strings.HasPrefix(c.GetHeader(constants.HeaderAuthorization), "Bearer ") {
			c.Next()
			return
		}

		// Skip for safe methods
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Apply CSRF only for cookie-based auth
		csrfCookie, err := c.Cookie(constants.CookieCSRF)
		csrfHeader := c.GetHeader(constants.HeaderCSRF)
		if err != nil || !csrfService.ValidateToken(csrfCookie, csrfHeader) {
			utils.HandleAPIError(c, nil, common.ErrCodeForbidden, "CSRF token required for cookie-based authentication")
			c.Abort()
			return
		}
		c.Next()
	}
}
