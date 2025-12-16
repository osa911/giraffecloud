package middleware

import (
	"strings"

	"github.com/osa911/giraffecloud/internal/api/constants"
	"github.com/osa911/giraffecloud/internal/api/dto/common"
	"github.com/osa911/giraffecloud/internal/config/firebase"
	"github.com/osa911/giraffecloud/internal/db/ent"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/repository"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware handles authentication and authorization
type AuthMiddleware struct {
	tokenService *service.TokenService
	authRepo     repository.AuthRepository
	sessionRepo  repository.SessionRepository
	userRepo     repository.UserRepository
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(
	tokenService *service.TokenService,
	authRepo repository.AuthRepository,
	sessionRepo repository.SessionRepository,
	userRepo repository.UserRepository,
) *AuthMiddleware {
	return &AuthMiddleware{
		tokenService: tokenService,
		authRepo:     authRepo,
		sessionRepo:  sessionRepo,
		userRepo:     userRepo,
	}
}

// RequireAuth middleware
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var currentUser *ent.User
		var authenticated bool

		var logger = logging.GetGlobalLogger()
		// First check for session cookie (Firebase session cookie)
		sessionCookie, err := c.Cookie(constants.CookieSession)
		logger.Info("sessionCookie: %v", sessionCookie)
		if err == nil && sessionCookie != "" {
			// Verify the session cookie
			firebaseToken, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
			logger.Info("firebaseToken: %v", firebaseToken)
			logger.Info("err: %v", err)
			if err == nil {
				// Look up user by Firebase UID
				currentUser, err = m.authRepo.GetUserByFirebaseUID(c.Request.Context(), firebaseToken.UID)
				logger.Info("currentUser: %v", currentUser)
				logger.Info("err: %v", err)
				if err == nil {
					authenticated = true
				}
			}
		}

		// If not authenticated, check for auth_token cookie (our API token)
		if !authenticated {
			cookieAuthToken, err := c.Cookie(constants.CookieAuthToken)
			logger.Info("cookieAuthToken: %v", cookieAuthToken)
			logger.Info("err: %v", err)
			if err == nil && cookieAuthToken != "" {
				// Look up session
				currentSession, err := m.sessionRepo.GetActiveByToken(c.Request.Context(), cookieAuthToken)
				logger.Info("currentSession: %v", currentSession)
				logger.Info("err: %v", err)
				if err == nil {
					// Update session last used
					_, err = m.sessionRepo.UpdateLastUsed(c.Request.Context(), currentSession, nil)
					logger.Info("err: %v", err)
					if err != nil {
						utils.LogError(err, "Failed to update session last used time")
					}

					currentUser, err = m.sessionRepo.GetSessionOwner(c.Request.Context(), currentSession)
					logger.Info("currentUser: %v", currentUser)
					logger.Info("err: %v", err)
					if err == nil {
						authenticated = true
					}
				}
			}
		}

		// If not authenticated, check for Bearer token in Authorization header (CLI token)
		if !authenticated {
			cliAuthHeader := c.GetHeader(constants.HeaderAuthorization)
			logger.Info("cliAuthHeader: %v", cliAuthHeader)
			if strings.HasPrefix(cliAuthHeader, "Bearer ") {
				token := strings.TrimPrefix(cliAuthHeader, "Bearer ")

				// Validate token using TokenService
				cliTokenRecord, err := m.tokenService.ValidateToken(c.Request.Context(), token)
				logger.Info("cliTokenRecord: %v", cliTokenRecord)
				logger.Info("err: %v", err)
				if err == nil {
					// Get user from token using UserRepository
					currentUser, err = m.userRepo.Get(c.Request.Context(), cliTokenRecord.UserID)
					logger.Info("currentUser: %v", currentUser)
					logger.Info("err: %v", err)
					if err == nil {
						authenticated = true
					}
				}
			}
		}

		// If user was not authenticated by any method
		if !authenticated {
			// Clear cookies to prevent redirect loops
			cookieDomain := utils.GetCookieDomain()
			c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, cookieDomain, true, true)
			c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathRoot, cookieDomain, true, true)
			c.SetCookie(constants.CookieCSRF, "", -1, constants.CookiePathRoot, cookieDomain, true, false)

			utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Authentication required, please log in again")
			c.Abort()
			return
		}

		// Update last login info using AuthRepository
		currentUser, err = m.authRepo.UpdateUserLastLogin(c.Request.Context(), currentUser, utils.GetRealIP(c))
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
