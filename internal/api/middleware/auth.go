package middleware

import (
	"net/http"
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/models"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware handles authentication and authorization
type AuthMiddleware struct {}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// RequireAuth middleware
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user models.User
		var authenticated bool

		// First check for session cookie (Firebase session cookie)
		sessionCookie, err := c.Cookie(constants.CookieSession)
		if err == nil && sessionCookie != "" {
			// Verify the session cookie
			token, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
			if err == nil {
				// Look up user by Firebase UID
				if err := db.DB.Where("firebase_uid = ?", token.UID).First(&user).Error; err == nil {
					authenticated = true
				}
			}
		}

		// If not authenticated, check for auth_token cookie (our API token)
		if !authenticated {
			authToken, err := c.Cookie(constants.CookieAuthToken)
			if err == nil && authToken != "" {
				// Look up session
				var session models.Session
				if err := db.DB.Where("token = ? AND is_active = ? AND expires_at > ?",
					authToken, true, time.Now()).First(&session).Error; err == nil {

					// Update session last used
					session.LastUsed = time.Now()
					db.DB.Save(&session)

					// Lookup user
					if err := db.DB.First(&user, session.UserID).Error; err == nil {
						authenticated = true
					}
				}
			}
		}

		// If user was not authenticated by any method
		if !authenticated {
			response := common.NewErrorResponse(common.ErrCodeUnauthorized, "Authentication required, please log in again", nil)
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}

		// Update last login info
		user.LastLogin = time.Now()
		user.LastLoginIP = utils.GetRealIP(c)
		if err := db.DB.Save(&user).Error; err != nil {
			response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update user", err)
			c.JSON(http.StatusInternalServerError, response)
			c.Abort()
			return
		}

		// Set user and userID in context
		c.Set(constants.ContextKeyUserID, user.ID)
		c.Set(constants.ContextKeyUser, user)
		c.Next()
	}
}

// RequireAdmin is a middleware that ensures a user is an admin
func (m *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First check if user is authenticated
		m.RequireAuth()(c)

		// If there was an error, RequireAuth would have aborted the chain
		if c.IsAborted() {
			return
		}

		// Get user from context
		user, exists := c.Get(constants.ContextKeyUser)
		if !exists {
			response := common.NewErrorResponse(common.ErrCodeInternalServer, "User not found in context after auth", nil)
			c.JSON(http.StatusInternalServerError, response)
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			response := common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid user type in context", nil)
			c.JSON(http.StatusInternalServerError, response)
			c.Abort()
			return
		}

		if u.Role != models.RoleAdmin {
			response := common.NewErrorResponse(common.ErrCodeForbidden, "Admin access required", nil)
			c.JSON(http.StatusForbidden, response)
			c.Abort()
			return
		}

		c.Next()
	}
}