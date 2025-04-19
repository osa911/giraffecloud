package middleware

import (
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/session"
	"giraffecloud/internal/db/ent/user"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware handles authentication and authorization
type AuthMiddleware struct{}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// RequireAuth middleware
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var currentUser *ent.User
		var authenticated bool

		// First check for session cookie (Firebase session cookie)
		sessionCookie, err := c.Cookie(constants.CookieSession)
		if err == nil && sessionCookie != "" {
			// Verify the session cookie
			token, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
			if err == nil {
				// Look up user by Firebase UID
				currentUser, err = db.Client.User.Query().
					Where(user.FirebaseUID(token.UID)).
					Only(c.Request.Context())
				if err == nil {
					authenticated = true
				}
			}
		}

		// If not authenticated, check for auth_token cookie (our API token)
		if !authenticated {
			authToken, err := c.Cookie(constants.CookieAuthToken)
			if err == nil && authToken != "" {
				// Look up session
				currentSession, err := db.Client.Session.Query().
					Where(
						session.Token(authToken),
						session.IsActive(true),
						session.ExpiresAtGT(time.Now()),
					).
					WithOwner().
					Only(c.Request.Context())
				if err == nil {
					// Update session last used
					_, err = db.Client.Session.UpdateOne(currentSession).
						SetLastUsed(time.Now()).
						Save(c.Request.Context())
					if err != nil {
						utils.LogError(err, "Failed to update session last used time")
					}

					currentUser = currentSession.Edges.Owner
					if currentUser != nil {
						authenticated = true
					}
				}
			}
		}

		// If user was not authenticated by any method
		if !authenticated {
			utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Authentication required, please log in again")
			c.Abort()
			return
		}

		// Update last login info
		_, err = db.Client.User.UpdateOne(currentUser).
			SetLastLogin(time.Now()).
			Save(c.Request.Context())
		if err != nil {
			utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to update user")
			c.Abort()
			return
		}

		// Set user and userID in context
		c.Set(constants.ContextKeyUserID, currentUser.ID)
		c.Set(constants.ContextKeyUser, currentUser)
		c.Next()
	}
}