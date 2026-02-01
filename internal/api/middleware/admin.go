package middleware

import (
	"github.com/osa911/giraffecloud/internal/api/constants"
	"github.com/osa911/giraffecloud/internal/api/dto/common"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	// RoleAdmin is the admin role value
	RoleAdmin = "admin"
)

// AdminMiddleware handles admin-only authorization
type AdminMiddleware struct{}

// NewAdminMiddleware creates a new admin middleware
func NewAdminMiddleware() *AdminMiddleware {
	return &AdminMiddleware{}
}

// RequireAdmin middleware ensures the authenticated user has admin role
// This should be used AFTER RequireAuth middleware
func (m *AdminMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.GetGlobalLogger()

		// Get user from context (set by RequireAuth middleware)
		userModel, exists := c.Get(constants.ContextKeyUser)
		if !exists {
			logger.Warn("Admin access attempted without authenticated user")
			utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Authentication required")
			c.Abort()
			return
		}

		user, ok := userModel.(*ent.User)
		if !ok {
			logger.Error("Invalid user type in context during admin check")
			utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Internal server error")
			c.Abort()
			return
		}

		// Check if user has admin role
		if user.Role != RoleAdmin {
			logger.Warn("Non-admin user attempted to access admin resource: userID=%d email=%s role=%s",
				user.ID, user.Email, user.Role)
			utils.HandleAPIError(c, nil, common.ErrCodeForbidden, "Admin access required")
			c.Abort()
			return
		}

		logger.Debug("Admin access granted for user: %d", user.ID)
		c.Next()
	}
}
