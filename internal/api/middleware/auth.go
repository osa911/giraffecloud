package middleware

import (
	"net/http"
	"strings"
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/models"

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
		// Check for session cookie
		sessionCookie, err := c.Cookie("session")
		var uid string

		if err == nil && sessionCookie != "" {
			// Verify the session cookie
			sessionToken, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
			if err == nil {
				uid = sessionToken.UID
			}
			// Continue to try the Authorization header if session cookie is invalid
		}

		// If no valid session cookie, try Authorization header
		if uid == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				response := common.NewErrorResponse(common.ErrCodeUnauthorized, "Authentication required", nil)
				c.JSON(http.StatusUnauthorized, response)
				c.Abort()
				return
			}

			// Extract token from Bearer header
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				response := common.NewErrorResponse(common.ErrCodeUnauthorized, "Invalid authorization header format", nil)
				c.JSON(http.StatusUnauthorized, response)
				c.Abort()
				return
			}

			token := parts[1]
			tokenLen := len(token)
			if tokenLen < 20 {
				response := common.NewErrorResponse(common.ErrCodeUnauthorized, "Invalid token format: token too short", nil)
				c.JSON(http.StatusUnauthorized, response)
				c.Abort()
				return
			}

			// Verify the Firebase token
			uid, err = firebase.VerifyToken(c.Request.Context(), token)
			if err != nil {
				response := common.NewErrorResponse(common.ErrCodeUnauthorized, "Invalid token: "+err.Error(), nil)
				c.JSON(http.StatusUnauthorized, response)
				c.Abort()
				return
			}
		}

		// Get user from database
		var user models.User
		result := db.DB.Where("firebase_uid = ?", uid).First(&user)
		if result.Error != nil {
			response := common.NewErrorResponse(common.ErrCodeUnauthorized, "User not found in database", gin.H{
				"firebase_uid": uid,
				"message":      "You are authenticated with Firebase, but your user record is not found in our database. Please try logging in again.",
			})
			c.JSON(http.StatusUnauthorized, response)
			c.Abort()
			return
		}

		// Update last login info
		user.LastLogin = time.Now()
		user.LastLoginIP = c.ClientIP()
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