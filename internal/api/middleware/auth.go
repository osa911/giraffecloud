package middleware

import (
	"net/http"
	"strings"
	"time"

	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/models"

	"github.com/gin-gonic/gin"
)

// RequireAuth middleware
func RequireAuth() gin.HandlerFunc {
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
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}

			// Extract token from Bearer header
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
				c.Abort()
				return
			}

			token := parts[1]
			tokenLen := len(token)
			if tokenLen < 20 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format: token too short"})
				c.Abort()
				return
			}

			// Verify the Firebase token
			uid, err = firebase.VerifyToken(c.Request.Context(), token)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
				c.Abort()
				return
			}
		}

		// Get user from database
		var user models.User
		result := db.DB.Where("firebase_uid = ?", uid).First(&user)
		if result.Error != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":        "User not found in database",
				"firebase_uid": uid,
				"code":         "USER_NOT_FOUND",
				"message":      "You are authenticated with Firebase, but your user record is not found in our database. Please try logging in again.",
			})
			c.Abort()
			return
		}

		// Update last login info
		user.LastLogin = time.Now()
		user.LastLoginIP = c.ClientIP()
		if err := db.DB.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Next()
	}
}

// RequireAdmin middleware
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			c.Abort()
			return
		}

		u, ok := user.(models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type in context"})
			c.Abort()
			return
		}

		if u.Role != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}