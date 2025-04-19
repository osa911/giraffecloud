package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authRepo     repository.AuthRepository
	sessionRepo  repository.SessionRepository
	csrfService  service.CSRFService
	auditService *service.AuditService
}

func NewAuthHandler(
	authRepo repository.AuthRepository,
	sessionRepo repository.SessionRepository,
	csrfService service.CSRFService,
	auditService *service.AuditService,
) *AuthHandler {
	return &AuthHandler{
		authRepo:     authRepo,
		sessionRepo:  sessionRepo,
		csrfService:  csrfService,
		auditService: auditService,
	}
}

// generateSecureToken creates a secure random token
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CheckIfIP checks if a string is an IP address
func isIPAddress(host string) bool {
	// Simple check for IPv4 - looks for 4 segments of numbers separated by dots
	ipv4Parts := strings.Split(host, ".")
	if len(ipv4Parts) == 4 {
		for _, part := range ipv4Parts {
			// Check if each part contains only digits
			if !containsOnlyDigits(part) {
				return false
			}
		}
		return true
	}
	// Check for presence of colons which suggests IPv6
	return strings.Contains(host, ":")
}

// Helper function to check if a string contains only digits
func containsOnlyDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Get the appropriate cookie domain based on environment
func getCookieDomain() string {
	env := os.Getenv("ENV")
	clientURL := os.Getenv("CLIENT_URL")

	if env == "production" && clientURL != "" {
		parsableURL := clientURL
		if !strings.HasPrefix(parsableURL, "http://") && !strings.HasPrefix(parsableURL, "https://") {
			parsableURL = "https://" + parsableURL
		}

		parsedURL, err := url.Parse(parsableURL)
		if err != nil {
			return ""
		}

		host := parsedURL.Hostname()

		if host == "" {
			return clientURL
		}

		if host == "localhost" || host == "127.0.0.1" || isIPAddress(host) {
			return ""
		}

		parts := strings.Split(host, ".")
		if len(parts) >= 2 {
			domain := parts[len(parts)-2] + "." + parts[len(parts)-1]
			return "." + domain
		} else if host != "" {
			return host
		}
	}

	return "" // Default empty string
}

func (h *AuthHandler) Login(c *gin.Context) {
	logger := logging.GetLogger()

	// Get login data from context (set by ValidateLoginRequest middleware)
	loginData, exists := c.Get(constants.ContextKeyLogin)
	if !exists {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Login data not found in context. Ensure validation middleware is applied.", nil))
		return
	}

	// Extract token from login data
	loginPtr, ok := loginData.(*auth.LoginRequest)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid login data format", nil))
		return
	}

	// Verify the Firebase token
	decodedToken, err := firebase.GetAuthClient().VerifyIDToken(c.Request.Context(), loginPtr.Token)
	if err != nil {
		h.auditService.LogFailedAuthAttempt(
			c.Request.Context(),
			utils.GetRealIP(c),
			"Invalid Firebase token",
			map[string]interface{}{
				"error": err.Error(),
			},
		)
		utils.HandleAPIError(c, err, http.StatusUnauthorized, common.ErrCodeUnauthorized, "Invalid token")
		return
	}

	// Create a new session cookie with the verified ID token
	expiresIn := time.Hour * 24 * 7 // 7 days for the session cookie
	sessionCookie, err := firebase.GetAuthClient().SessionCookie(c.Request.Context(), loginPtr.Token, expiresIn)
	if err != nil {
		h.auditService.LogFailedAuthAttempt(
			c.Request.Context(),
			utils.GetRealIP(c),
			"Failed to create session cookie",
			map[string]interface{}{
				"error": err.Error(),
			},
		)
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create session cookie")
		return
	}

	// Set the Firebase session cookie
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		constants.CookieSession,
		sessionCookie,
		constants.CookieDurationWeek,
		constants.CookiePathRoot,
		getCookieDomain(),
		true,
		true,
	)

	// Check if user exists in database with this Firebase UID
	existingUser, err := h.authRepo.GetUserByFirebaseUID(c.Request.Context(), decodedToken.UID)
	if err != nil {
		if ent.IsNotFound(err) {
			// User not found in our database, but authenticated with Firebase
			// Create a new user with minimal data from Firebase
			email := decodedToken.Claims["email"].(string)
			name := email
			if fullName, ok := decodedToken.Claims["name"].(string); ok && fullName != "" {
				name = fullName
			}

			// Create new user
			existingUser, err = h.authRepo.CreateUser(c.Request.Context(), decodedToken.UID, email, name, utils.GetRealIP(c))
			if err != nil {
				h.auditService.LogFailedAuthAttempt(
					c.Request.Context(),
					utils.GetRealIP(c),
					"Failed to create user",
					map[string]interface{}{
						"error": err.Error(),
						"email": email,
					},
				)
				logger.Error("Failed to create user: %v", err)
				utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create user")
				return
			}
		} else {
			h.auditService.LogFailedAuthAttempt(
				c.Request.Context(),
				utils.GetRealIP(c),
				"Database error",
				map[string]interface{}{
					"error": err.Error(),
				},
			)
			utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database error")
			return
		}
	}

	// Update last login info
	existingUser, err = h.authRepo.UpdateUserLastLogin(c.Request.Context(), existingUser, utils.GetRealIP(c))
	if err != nil {
		h.auditService.LogFailedAuthAttempt(
			c.Request.Context(),
			utils.GetRealIP(c),
			"Failed to update user",
			map[string]interface{}{
				"error": err.Error(),
				"user_id": existingUser.ID,
			},
		)
		logger.Error("Failed to update user: %v", err)
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user")
		return
	}

	// Get user agent and IP address
	userAgent := c.GetHeader("User-Agent")
	if userAgent == "" {
		userAgent = "Unknown"
	}
	ipAddress := utils.GetRealIP(c)

	// Generate a secure session token
	sessionToken, err := generateSecureToken(64)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to generate session token")
		return
	}

	// Create and store the session
	expiresAt := time.Now().Add(time.Hour * 24 * 30) // 30 days
	session, err := h.sessionRepo.CreateForUser(c.Request.Context(), existingUser.ID, sessionToken, userAgent, ipAddress, expiresAt)
	if err != nil {
		h.auditService.LogFailedAuthAttempt(
			c.Request.Context(),
			utils.GetRealIP(c),
			"Failed to create session",
			map[string]interface{}{
				"error": err.Error(),
				"user_id": existingUser.ID,
			},
		)
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create session")
		return
	}

	// Get the session with owner loaded for audit logging
	sessionWithOwner, err := h.sessionRepo.GetActiveByToken(c.Request.Context(), session.Token)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to load session details")
		return
	}

	// Log successful login and session creation
	h.auditService.LogAuthEvent(
		c.Request.Context(),
		service.AuditEventLogin,
		existingUser,
		utils.GetRealIP(c),
		map[string]interface{}{
			"user_agent": userAgent,
			"ip_address": ipAddress,
		},
	)
	h.auditService.LogSessionEvent(
		c.Request.Context(),
		service.AuditEventSessionCreated,
		sessionWithOwner,
		nil,
	)

	// Set cookie domain based on environment
	cookieDomain := getCookieDomain()

	// Set the session cookie (client-side)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		constants.CookieAuthToken,
		session.Token,
		constants.CookieDuration30d, // 30 days
		constants.CookiePathAPI,
		cookieDomain,
		true,  // Secure
		true,  // HttpOnly
	)

	// Generate and set CSRF token cookie (not HttpOnly)
	csrfToken, err := h.csrfService.GenerateToken()
	if err == nil {
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			constants.CookieCSRF,
			csrfToken,
			constants.CookieDuration30d,
			constants.CookiePathAPI,
			cookieDomain,
			true, // Secure
			false, // NOT HttpOnly
		)
	}

	// Return user data and session info
	userResponse := mapper.UserToUserResponse(existingUser)
	c.JSON(http.StatusOK, common.NewSuccessResponse(auth.LoginResponse{
		User: *userResponse,
	}))
}

func (h *AuthHandler) Register(c *gin.Context) {
	// Get registration data from context (set by ValidateRegisterRequest middleware)
	registerData, exists := c.Get(constants.ContextKeyRegister)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Registration data not found in context. Ensure validation middleware is applied.")
		return
	}

	// Extract and convert to RegisterRequest
	registerPtr, ok := registerData.(*auth.RegisterRequest)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid registration data format")
		return
	}

	// Check if user exists with the same Firebase UID
	existingUser, err := h.authRepo.GetUserByFirebaseUID(c.Request.Context(), registerPtr.Token)
	if err == nil {
		// User exists, return the existing user
		userResponse := mapper.UserToUserResponse(existingUser)
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.RegisterResponse{
			User: *userResponse,
		}))
		return
	} else if !ent.IsNotFound(err) {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database error")
		return
	}

	// Check if user exists with the same email
	existingUser, err = h.authRepo.GetUserByEmail(c.Request.Context(), registerPtr.Email)
	if err == nil {
		utils.HandleAPIError(c, nil, http.StatusConflict, common.ErrCodeConflict, "Wrong credentials")
		return
	} else if !ent.IsNotFound(err) {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database error")
		return
	}

	// Create new user
	newUser, err := h.authRepo.CreateUser(c.Request.Context(), registerPtr.Token, registerPtr.Email, registerPtr.Name, utils.GetRealIP(c))
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create user")
		return
	}

	// Return response using mapper with proper DTO format
	userResponse := mapper.UserToUserResponse(newUser)
	c.JSON(http.StatusCreated, common.NewSuccessResponse(auth.RegisterResponse{
		User: *userResponse,
	}))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	logger := logging.GetLogger()

	// Get user from context
	user, _ := c.Get(constants.ContextKeyUser)
	currentUser, ok := user.(*ent.User)

	// Check for session cookie to identify and invalidate server-side session
	sessionCookie, err := c.Cookie(constants.CookieAuthToken)
	if err == nil && sessionCookie != "" {
		// Get session before invalidating it (for audit log)
		session, err := h.sessionRepo.GetActiveByToken(c.Request.Context(), sessionCookie)
		if err == nil {
			// Log session revocation
			h.auditService.LogSessionEvent(
				c.Request.Context(),
				service.AuditEventSessionRevoked,
				session,
				nil,
			)
		}

		// Invalidate the session in the database
		err = h.sessionRepo.RevokeByToken(c.Request.Context(), sessionCookie)
		if err != nil {
			logger.Error("Failed to invalidate session: %v", err)
			utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to invalidate session")
			return
		}
	}

	// Log logout event if we have user info
	if ok && currentUser != nil {
		h.auditService.LogAuthEvent(
			c.Request.Context(),
			service.AuditEventLogout,
			currentUser,
			utils.GetRealIP(c),
			nil,
		)
	}

	// Clear the session cookies
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, getCookieDomain(), true, true)
	c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathAPI, getCookieDomain(), true, true)

	// Return success response
	c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
}

func (h *AuthHandler) GetSession(c *gin.Context) {
	// Check for session cookie first
	sessionCookie, err := c.Cookie(constants.CookieSession)
	if err == nil && sessionCookie != "" {
		// Verify the session cookie
		token, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
		if err == nil {
			// Session cookie is valid
			existingUser, err := h.authRepo.GetUserByFirebaseUID(c.Request.Context(), token.UID)
			if err == nil {
				// Update user's last activity
				existingUser, err = h.authRepo.UpdateUserLastActivity(c.Request.Context(), existingUser)
				if err != nil {
					utils.LogError(err, "Failed to update user's last activity")
					utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user's last activity")
					return
				}

				// Return session response using proper DTO format
				userResponse := mapper.UserToUserResponse(existingUser)
				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionValidationResponse{
					Valid: true,
					User:  userResponse,
				}))
				return
			}
		}
	}

	// Check auth_token cookie as fallback
	authToken, err := c.Cookie(constants.CookieAuthToken)
	if err == nil && authToken != "" {
		existingSession, err := h.sessionRepo.GetActiveByToken(c.Request.Context(), authToken)
		if err == nil {
			// Update session last used
			existingSession, err = h.sessionRepo.UpdateLastUsed(c.Request.Context(), existingSession, nil)
			if err != nil {
				utils.LogError(err, "Failed to update session's last used time")
				utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update session's last used time")
				return
			}

			// Refresh the cookie with reset expiration
			c.SetSameSite(http.SameSiteStrictMode)
			c.SetCookie(
				constants.CookieAuthToken,
				authToken,
				constants.CookieDuration24h,
				constants.CookiePathAPI,
				getCookieDomain(),
				true,
				true,
			)

			// Fetch user
			owner, err := h.sessionRepo.GetSessionOwner(c.Request.Context(), existingSession)
			if err == nil {
				// Return session response using proper DTO format
				userResponse := mapper.UserToUserResponse(owner)
				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionValidationResponse{
					Valid: true,
					User:  userResponse,
				}))
				return
			}
		}
	}

	// If we get here, no valid session was found
	c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionValidationResponse{
		Valid: false,
	}))
}

func (h *AuthHandler) RefreshSession(c *gin.Context) {
	var authenticated bool
	var existingUser *ent.User

	// Check for session cookie first
	sessionCookie, err := c.Cookie(constants.CookieSession)
	if err == nil && sessionCookie != "" {
		// Verify the session cookie
		decodedToken, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
		if err == nil {
			// Get the user from Firebase UID
			existingUser, err = h.authRepo.GetUserByFirebaseUID(c.Request.Context(), decodedToken.UID)
			if err == nil {
				authenticated = true
			}
		}
	}

	// Check auth_token cookie as fallback
	if !authenticated {
		authToken, err := c.Cookie(constants.CookieAuthToken)
		if err == nil && authToken != "" {
			existingSession, err := h.sessionRepo.GetActiveByToken(c.Request.Context(), authToken)
			if err == nil {
				// Update session last used & extend expiration time
				newExpiration := time.Now().Add(time.Hour * 24 * 30) // Reset to full 30 days
				existingSession, err = h.sessionRepo.UpdateLastUsed(c.Request.Context(), existingSession, &newExpiration)
				if err != nil {
					utils.LogError(err, "Failed to update session")
					utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update session")
					return
				}

				// Refresh the auth_token cookie
				c.SetSameSite(http.SameSiteStrictMode)
				c.SetCookie(
					constants.CookieAuthToken,
					authToken,
					constants.CookieDuration24h,
					constants.CookiePathAPI,
					getCookieDomain(),
					true,
					true,
				)

				if !authenticated {
					// Get the user if we haven't already
					owner, err := h.sessionRepo.GetSessionOwner(c.Request.Context(), existingSession)
					if err == nil {
						existingUser = owner
						authenticated = true
					}
				}
			}
		}
	}

	if authenticated && existingUser != nil {
		// Update last activity
		existingUser, err = h.authRepo.UpdateUserLastActivity(c.Request.Context(), existingUser)
		if err != nil {
			utils.LogError(err, "Failed to update user's last activity")
			utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user's last activity")
			return
		}

		c.JSON(http.StatusOK, common.NewMessageResponse("Session valid"))
	} else {
		c.JSON(http.StatusUnauthorized, common.NewErrorResponse(common.ErrCodeUnauthorized, "No valid session found", nil))
	}
}

func (h *AuthHandler) VerifyToken(c *gin.Context) {
	// Get ID token from context (set by validation middleware)
	verifyData, exists := c.Get(constants.ContextKeyVerifyToken)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusNotFound, common.ErrCodeNotFound, "Verification data not found in context")
		return
	}

	// Extract token data
	verifyReq, ok := verifyData.(*auth.VerifyTokenRequest)
	if !ok {
		utils.LogError(fmt.Errorf("invalid verification data type: %T", verifyData), "Invalid verification data format")
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid verification data format")
		return
	}

	// Verify the ID token
	decodedToken, err := firebase.GetAuthClient().VerifyIDToken(c.Request.Context(), verifyReq.IDToken)
	if err != nil {
		utils.LogError(err, "Failed to verify Firebase ID token")
		utils.HandleAPIError(c, err, http.StatusUnauthorized, common.ErrCodeUnauthorized, "Invalid ID token")
		return
	}

	// Create a new session cookie with the verified ID token
	expiresIn := time.Hour * 24 * 7 // 7 days for the session cookie
	sessionCookie, err := firebase.GetAuthClient().SessionCookie(c.Request.Context(), verifyReq.IDToken, expiresIn)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create session cookie")
		return
	}

	// Set the new session cookie
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		constants.CookieSession,
		sessionCookie,
		constants.CookieDurationWeek,
		constants.CookiePathRoot,
		getCookieDomain(),
		true,
		true,
	)

	// Look up user to include in response
	existingUser, err := h.authRepo.GetUserByFirebaseUID(c.Request.Context(), decodedToken.UID)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to find user")
		return
	}

	// Update last activity
	existingUser, err = h.authRepo.UpdateUserLastActivity(c.Request.Context(), existingUser)
	if err != nil {
		utils.LogError(err, "Failed to update user's last activity")
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user's last activity")
		return
	}

	// Return success response
	userResponse := mapper.UserToUserResponse(existingUser)
	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
		"message": "Session refreshed successfully",
		"user":    userResponse,
	}))
}