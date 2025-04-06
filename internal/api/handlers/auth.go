package handlers

import (
	"net/http"
	"strings"
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

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
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
		c.JSON(http.StatusConflict, common.NewErrorResponse(common.ErrCodeConflict, "Email already registered", nil))
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
	// Check for an authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// No token provided, still return success since user is effectively logged out
		c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
		return
	}

	// Extract token from Bearer header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		// Invalid token format, still return success
		c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
		return
	}

	token := parts[1]

	// Try to verify the token to get the user ID
	uid, err := firebase.VerifyToken(c.Request.Context(), token)
	if err != nil {
		// Token verification failed, still return success
		c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
		return
	}

	// Get user from database
	var user models.User
	result := h.db.Where("firebase_uid = ?", uid).First(&user)
	if result.Error == nil {
		// Update user's last_activity field
		h.db.Model(&user).Updates(map[string]interface{}{
			"last_activity": time.Now(),
		})
	}

	c.JSON(http.StatusOK, common.NewMessageResponse("Logged out successfully"))
}

// GetSession checks if the user has a valid session
func (h *AuthHandler) GetSession(c *gin.Context) {
	// Get Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// No token provided
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
			Valid: false,
		}))
		return
	}

	// Extract token from Bearer header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
			Valid: false,
		}))
		return
	}

	token := parts[1]

	// Try to verify the token
	uid, err := firebase.VerifyToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
			Valid: false,
		}))
		return
	}

	// Get user from database
	var user models.User
	result := h.db.Where("firebase_uid = ?", uid).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
			Valid: false,
		}))
		return
	}

	// Update user's last_activity field
	h.db.Model(&user).Updates(map[string]interface{}{
		"last_activity": time.Now(),
	})

	// Return session response using proper DTO format
	userResponse := mapper.UserToAuthUserResponse(&user)
	c.JSON(http.StatusOK, common.NewSuccessResponse(auth.SessionResponse{
		Valid: true,
		User:  userResponse,
	}))
}