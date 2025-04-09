package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/models"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
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

func (h *AuthHandler) Login(c *gin.Context) {
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
		utils.HandleAPIError(c, err, http.StatusUnauthorized, common.ErrCodeUnauthorized, "Invalid token")
		return
	}

	// Check if user exists in database with this Firebase UID
	var user models.User
	result := h.db.Where("firebase_uid = ?", decodedToken.UID).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// User not found in our database, but authenticated with Firebase
			// Create a new user with minimal data from Firebase
			email := decodedToken.Claims["email"].(string)
			name := email
			if fullName, ok := decodedToken.Claims["name"].(string); ok && fullName != "" {
				name = fullName
			}

			// Create new user model
			user = models.User{
				FirebaseUID: decodedToken.UID,
				Email:      email,
				Name:       name,
				Role:       models.RoleUser,
				IsActive:   true,
				LastLogin:  time.Now(),
				LastLoginIP: utils.GetRealIP(c),
			}

			if err := h.db.Create(&user).Error; err != nil {
				utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create user")
				return
			}
		} else {
			utils.HandleAPIError(c, result.Error, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database error")
			return
		}
	}

	// Update last login info
	user.LastLogin = time.Now()
	user.LastLoginIP = utils.GetRealIP(c)
	if err := h.db.Save(&user).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user")
		return
	}

	// Create a server-side session
	deviceName := "Unknown"
	userAgent := c.GetHeader("User-Agent")
	if userAgent != "" {
		deviceName = userAgent
	}

	// Generate a unique device ID if not provided
	deviceID, err := generateSecureToken(32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to generate session token")
		return
	}

	// Generate a secure session token
	sessionToken, err := generateSecureToken(64)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to generate session token")
		return
	}

	// Create and store the session in the database with longer expiration (server-side)
	session := models.Session{
		UserID:     user.ID,
		Token:      sessionToken,
		DeviceName: deviceName,
		DeviceID:   deviceID,
		LastUsed:   time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour * 24 * 30), // 30 days (server-side)
		IsActive:   true,
		IPAddress:  utils.GetRealIP(c),
		UserAgent:  userAgent,
	}

	if err := h.db.Create(&session).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create session")
		return
	}

	// Create Firebase session cookie with shorter lifetime
	// Use a shorter 24-hour expiration for client-side cookies to improve security
	// The cookies will be refreshed automatically when users visit the site
	expiresIn := time.Hour * 24 // 24 hours
	sessionCookie, err := firebase.GetAuthClient().SessionCookie(c.Request.Context(), loginPtr.Token, expiresIn)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create session cookie")
		return
	}

	// Set the session cookie - this will be used for authentication
	// HttpOnly prevents JavaScript access, Secure ensures HTTPS-only
	c.SetCookie(
		constants.CookieSession,
		sessionCookie,
		constants.CookieDuration24h,
		constants.CookiePathRoot,
		"", // Domain - leave empty for current domain
		true, // Secure - requires HTTPS (set to true in production)
		true, // HttpOnly - prevents JavaScript access
	)

	// Set API token cookie - for our backend API
	c.SetCookie(
		constants.CookieAuthToken,
		sessionToken,
		constants.CookieDuration24h,
		constants.CookiePathAPI,
		"",
		true,
		true,
	)

	// Return response using mapper and proper DTO format
	userResponse := mapper.UserToAuthUserResponse(&user)
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
	var existingFirebaseUser models.User
	if err := h.db.Where("firebase_uid = ?", registerPtr.FirebaseUID).First(&existingFirebaseUser).Error; err == nil {
		// Return the existing user using mapper with proper DTO format
		userResponse := mapper.UserToAuthUserResponse(&existingFirebaseUser)
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.RegisterResponse{
			User: *userResponse,
		}))
		return
	}

	// Check if user exists in our database with the same email
	var existingUser models.User
	if err := h.db.Where("email = ?", registerPtr.Email).First(&existingUser).Error; err == nil {
		utils.HandleAPIError(c, nil, http.StatusConflict, common.ErrCodeConflict, "Wrong credentials")
		return
	}

	// Create user in database
	user := models.User{
		FirebaseUID: registerPtr.FirebaseUID,
		Email:       registerPtr.Email,
		Name:        registerPtr.Name,
		Role:        models.RoleUser,
		IsActive:    true,
		LastLogin:   time.Now(),
		LastLoginIP: utils.GetRealIP(c),
	}

	if err := h.db.Create(&user).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create user")
		return
	}

	// Return response using mapper with proper DTO format
	userResponse := mapper.UserToAuthUserResponse(&user)
	c.JSON(http.StatusCreated, common.NewSuccessResponse(auth.RegisterResponse{
		User: *userResponse,
	}))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Clear the session cookie by setting an expired cookie
	c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, "", true, true)
	c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathAPI, "", true, true)

	// Check for session cookie to identify and invalidate server-side session
	sessionCookie, err := c.Cookie(constants.CookieAuthToken)
	if err == nil && sessionCookie != "" {
		// Invalidate the session in the database
		var session models.Session
		if err := h.db.Where("token = ?", sessionCookie).First(&session).Error; err == nil {
			session.IsActive = false
			h.db.Save(&session)
		}
	}

	// Return success response
	c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
}

// GetSession checks if the user has a valid session
func (h *AuthHandler) GetSession(c *gin.Context) {
	// Check for session cookie first
	sessionCookie, err := c.Cookie(constants.CookieSession)
	if err == nil && sessionCookie != "" {
		// Verify the session cookie
		token, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
		if err == nil {
			// Session cookie is valid
			var user models.User
			if err := h.db.Where("firebase_uid = ?", token.UID).First(&user).Error; err == nil {
				// Update user's last_activity field
				h.db.Model(&user).Updates(map[string]interface{}{
					"last_activity": time.Now(),
				})

				// Auto-refresh the cookie if it's going to expire soon (within 12 hours)
				// This requires checking the cookie expiration time
				// Since we can't directly check cookie expiration, we'll refresh auth_token
				// which will provide a fallback authentication method

				// Return session response using proper DTO format
				userResponse := mapper.UserToAuthUserResponse(&user)
				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
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
		var session models.Session
		if err := h.db.Where("token = ? AND is_active = ? AND expires_at > ?",
			authToken, true, time.Now()).First(&session).Error; err == nil {

			// Update session last used
			session.LastUsed = time.Now()
			h.db.Save(&session)

			// Refresh the cookie with reset expiration
			// This extends the client-side cookie lifetime
			c.SetCookie(
				constants.CookieAuthToken,
				authToken,
				constants.CookieDuration24h,
				constants.CookiePathAPI,
				"",
				true,
				true,
			)

			// Fetch user
			var user models.User
			if err := h.db.First(&user, session.UserID).Error; err == nil {
				// Return session response using proper DTO format
				userResponse := mapper.UserToAuthUserResponse(&user)
				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
					Valid: true,
					User:  userResponse,
				}))
				return
			}
		}
	}

	// If we get here, no valid session was found
	c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
		Valid: false,
	}))
}

// RefreshSession extends the session lifetime by refreshing cookies
func (h *AuthHandler) RefreshSession(c *gin.Context) {
	var user models.User
	var authenticated bool

	// Check for session cookie first
	sessionCookie, err := c.Cookie(constants.CookieSession)
	if err == nil && sessionCookie != "" {
		// Verify the session cookie
		decodedToken, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
		if err == nil {
			// Get the user from Firebase UID
			if err := h.db.Where("firebase_uid = ?", decodedToken.UID).First(&user).Error; err == nil {
				authenticated = true

				// Extend the session cookie if it will expire soon
				// This is now handled by the client using onIdTokenChanged
			}
		}
	}

	// Check auth_token cookie as fallback
	authToken, err := c.Cookie(constants.CookieAuthToken)
	if err == nil && authToken != "" {
		var session models.Session
		if err := h.db.Where("token = ? AND is_active = ? AND expires_at > ?",
			authToken, true, time.Now()).First(&session).Error; err == nil {

			// Update session last used & extend expiration time
			session.LastUsed = time.Now()
			session.ExpiresAt = time.Now().Add(time.Hour * 24 * 30) // Reset to full 30 days
			h.db.Save(&session)

			// Refresh the auth_token cookie
			c.SetCookie(
				constants.CookieAuthToken,
				authToken,
				constants.CookieDuration24h,
				constants.CookiePathAPI,
				"",
				true,
				true,
			)

			if !authenticated {
				// Get the user if we haven't already
				if err := h.db.First(&user, session.UserID).Error; err == nil {
					authenticated = true
				}
			}
		}
	}

	if authenticated {
		// The client will handle ID token refreshing via Firebase SDK
		// and call verify-token when needed
		c.JSON(http.StatusOK, common.NewMessageResponse("Session valid"))
	} else {
		c.JSON(http.StatusUnauthorized, common.NewErrorResponse(common.ErrCodeUnauthorized, "No valid session found", nil))
	}
}

// VerifyToken handles verification of a Firebase ID token and creates a new session cookie
func (h *AuthHandler) VerifyToken(c *gin.Context) {
	// Get ID token from context (set by validation middleware)
	verifyData, exists := c.Get(constants.ContextKeyVerifyToken)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Verification data not found in context")
		return
	}

	// Extract token data
	verifyReq, ok := verifyData.(*auth.VerifyTokenRequest)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid verification data format")
		return
	}

	// Verify the ID token
	decodedToken, err := firebase.GetAuthClient().VerifyIDToken(c.Request.Context(), verifyReq.IDToken)
	if err != nil {
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
	c.SetCookie(
		constants.CookieSession,
		sessionCookie,
		constants.CookieDurationWeek,
		constants.CookiePathRoot,
		"",
		true,
		true,
	)

	// Look up user to include in response
	var user models.User
	if err := h.db.Where("firebase_uid = ?", decodedToken.UID).First(&user).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to find user")
		return
	}

	// Update last activity
	user.LastActivity = time.Now()
	h.db.Save(&user)

	// Return success response
	userResponse := mapper.UserToAuthUserResponse(&user)
	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
		"message": "Session refreshed successfully",
		"user": userResponse,
	}))
}