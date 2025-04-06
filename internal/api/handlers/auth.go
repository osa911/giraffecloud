package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Simple in-memory session cache
type sessionCache struct {
	sync.RWMutex
	cache       map[string]*sessionCacheEntry
	initialized bool
}

type sessionCacheEntry struct {
	userResponse *auth.UserResponse
	expiry       time.Time
}

// Global session cache with 5-minute TTL
var sessionCacheStore = &sessionCache{
	cache: make(map[string]*sessionCacheEntry),
}

// getFromCache attempts to retrieve a cached session
func (sc *sessionCache) getFromCache(key string) *auth.UserResponse {
	sc.RLock()
	defer sc.RUnlock()

	if entry, found := sc.cache[key]; found {
		if time.Now().Before(entry.expiry) {
			return entry.userResponse
		}
		// Entry expired, remove it
		delete(sc.cache, key)
	}
	return nil
}

// addToCache adds a session to the cache with 2-minute TTL
func (sc *sessionCache) addToCache(key string, user *auth.UserResponse) {
	sc.Lock()
	defer sc.Unlock()

	sc.cache[key] = &sessionCacheEntry{
		userResponse: user,
		expiry:       time.Now().Add(2 * time.Minute),
	}

	// Clean expired entries periodically (every 100 adds)
	if !sc.initialized || len(sc.cache)%100 == 0 {
		go sc.cleanExpired()
		sc.initialized = true
	}
}

// cleanExpired removes expired entries from cache
func (sc *sessionCache) cleanExpired() {
	sc.Lock()
	defer sc.Unlock()

	now := time.Now()
	for key, entry := range sc.cache {
		if now.After(entry.expiry) {
			delete(sc.cache, key)
		}
	}
}

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
		c.JSON(http.StatusUnauthorized, common.NewErrorResponse(common.ErrCodeUnauthorized, "Invalid token", err))
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

			user = models.User{
				FirebaseUID: decodedToken.UID,
				Email:      email,
				Name:       name,
				Role:       models.RoleUser,
				IsActive:   true,
				LastLogin:  time.Now(),
				LastLoginIP: c.ClientIP(),
			}

			if err := h.db.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create user", err))
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Database error", result.Error))
			return
		}
	}

	// Update last login info
	user.LastLogin = time.Now()
	user.LastLoginIP = c.ClientIP()
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update user", err))
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
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to generate session token", err))
		return
	}

	// Generate a secure session token
	sessionToken, err := generateSecureToken(64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to generate session token", err))
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
		IPAddress:  c.ClientIP(),
		UserAgent:  userAgent,
	}

	if err := h.db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create session", err))
		return
	}

	// Create Firebase session cookie with shorter lifetime
	// Use a shorter 24-hour expiration for client-side cookies to improve security
	// The cookies will be refreshed automatically when users visit the site
	expiresIn := time.Hour * 24 // 24 hours
	sessionCookie, err := firebase.GetAuthClient().SessionCookie(c.Request.Context(), loginPtr.Token, expiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create session cookie", err))
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
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Registration data not found in context. Ensure validation middleware is applied.", nil))
		return
	}

	// Extract and convert to RegisterRequest
	registerPtr, ok := registerData.(*auth.RegisterRequest)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid registration data format", nil))
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
		c.JSON(http.StatusConflict, common.NewErrorResponse(common.ErrCodeConflict, "Wrong credentials", nil))
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
		LastLoginIP: c.ClientIP(),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create user", err))
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
	// Generate cache key from IP and auth tokens
	cacheKey := c.ClientIP()
	authToken, _ := c.Cookie(constants.CookieAuthToken)
	sessionCookie, _ := c.Cookie(constants.CookieSession)

	if authToken != "" {
		cacheKey += "-a-" + authToken[:10] // Use part of the token for the cache key
	}
	if sessionCookie != "" {
		cacheKey += "-s-" + sessionCookie[:10] // Use part of the session cookie
	}

	// First check our cache for a quick response
	if cachedUser := sessionCacheStore.getFromCache(cacheKey); cachedUser != nil {
		// We have a cached valid session, return immediately
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
			Valid: true,
			User:  cachedUser,
		}))
		return
	}

	// Cache miss - check session cookie
	if sessionCookie != "" {
		// Verify the session cookie
		token, err := firebase.GetAuthClient().VerifySessionCookieAndCheckRevoked(c.Request.Context(), sessionCookie)
		if err == nil {
			// Session cookie is valid - only select needed fields for efficiency
			var user models.User
			if err := h.db.Select("id, firebase_uid, email, name, role, is_active, last_activity").
				Where("firebase_uid = ?", token.UID).First(&user).Error; err == nil {

				// Only update last_activity if it's been more than 5 minutes
				if time.Since(user.LastActivity) > 5*time.Minute {
					h.db.Model(&user).UpdateColumn("last_activity", time.Now())
				}

				// Return and cache session response
				userResponse := mapper.UserToAuthUserResponse(&user)

				// Cache the valid session
				sessionCacheStore.addToCache(cacheKey, userResponse)

				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
					Valid: true,
					User:  userResponse,
				}))
				return
			}
		}
	}

	// Check auth_token cookie as fallback
	if authToken != "" {
		// First check if the session exists and is active
		type SessionData struct {
			ID       uint      `gorm:"column:id"`
			UserID   uint      `gorm:"column:user_id"`
			LastUsed time.Time `gorm:"column:last_used"`
		}

		var sessionData SessionData

		// Faster query by only selecting what we need and using index on token
		err := h.db.Table("sessions").
			Select("id, user_id, last_used").
			Where("token = ? AND is_active = ? AND expires_at > ?",
				authToken, true, time.Now()).
			First(&sessionData).Error

		if err == nil {
			// Update session last used - but only if it's been at least 5 minutes
			if time.Since(sessionData.LastUsed) > 5*time.Minute {
				h.db.Table("sessions").Where("id = ?", sessionData.ID).
					UpdateColumn("last_used", time.Now())
			}

			// Fetch user with just the needed fields
			var user models.User
			if err := h.db.Select("id, firebase_uid, email, name, role, is_active").
				First(&user, sessionData.UserID).Error; err == nil {

				// Return and cache session response
				userResponse := mapper.UserToAuthUserResponse(&user)

				// Cache the valid session
				sessionCacheStore.addToCache(cacheKey, userResponse)

				c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
					Valid: true,
					User:  userResponse,
				}))
				return
			}
		}
	}

	// If we get here, no valid session was found - also cache this negative result
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
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Verification data not found in context", nil))
		return
	}

	// Extract token data
	verifyReq, ok := verifyData.(*auth.VerifyTokenRequest)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid verification data format", nil))
		return
	}

	// Verify the ID token
	decodedToken, err := firebase.GetAuthClient().VerifyIDToken(c.Request.Context(), verifyReq.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, common.NewErrorResponse(common.ErrCodeUnauthorized, "Invalid ID token", err))
		return
	}

	// Create a new session cookie with the verified ID token
	expiresIn := time.Hour * 24 * 7 // 7 days for the session cookie
	sessionCookie, err := firebase.GetAuthClient().SessionCookie(c.Request.Context(), verifyReq.IDToken, expiresIn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create session cookie", err))
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
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to find user", err))
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