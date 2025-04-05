package handlers

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

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

type LoginRequest struct {
	Token string `json:"token" binding:"required"`
}

type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Name         string `json:"name" binding:"required"`
	FirebaseUID  string `json:"firebase_uid" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Make sure the token is present
	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Token is required",
		})
		return
	}

	// Verify the Firebase token
	decodedToken, err := firebase.GetAuthClient().VerifyIDToken(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	// Update last login info
	user.LastLogin = time.Now()
	user.LastLoginIP = c.ClientIP()
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"role":         user.Role,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	// Get registration data from context
	registerData, exists := c.Get("register")
	if !exists {
		// If not found in context, try to read from the request body
		// Read raw request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request body"})
			return
		}

		// Check if body is empty
		if len(body) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Empty request body"})
			return
		}

		// Restore the body so it can be read by ShouldBindJSON
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Use this request data
		registerData = req
	}

	// Convert to the correct type
	var req RegisterRequest
	if r, ok := registerData.(struct {
		Email        string `json:"email" binding:"required,email"`
		Name         string `json:"name" binding:"required"`
		FirebaseUID  string `json:"firebase_uid" binding:"required"`
	}); ok {
		req = RegisterRequest{
			Email:        r.Email,
			Name:         r.Name,
			FirebaseUID:  r.FirebaseUID,
		}
	} else if r, ok := registerData.(RegisterRequest); ok {
		req = r
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Check if user exists with the same Firebase UID
	var existingFirebaseUser models.User
	if err := h.db.Where("firebase_uid = ?", req.FirebaseUID).First(&existingFirebaseUser).Error; err == nil {
		// Return the existing user
		c.JSON(http.StatusOK, gin.H{
			"user": gin.H{
				"id":           existingFirebaseUser.ID,
				"email":        existingFirebaseUser.Email,
				"name":         existingFirebaseUser.Name,
				"role":         existingFirebaseUser.Role,
				"is_active":    existingFirebaseUser.IsActive,
			},
			"message": "User already registered",
		})
		return
	}

	// Check if user exists in our database with the same email
	var existingUser models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Create user in database
	user := models.User{
		FirebaseUID: req.FirebaseUID,
		Email:      req.Email,
		Name:       req.Name,
		Role:       models.RoleUser,
		IsActive:   true,
		LastLogin:  time.Now(),
		LastLoginIP: c.ClientIP(),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"role":         user.Role,
			"is_active":    user.IsActive,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Check for an authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// No token provided, still return success since user is effectively logged out
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
		return
	}

	// Extract token from Bearer header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		// Invalid token format, still return success
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
		return
	}

	token := parts[1]

	// Try to verify the token to get the user ID
	uid, err := firebase.VerifyToken(c.Request.Context(), token)
	if err != nil {
		// Token verification failed, still return success
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
		return
	}

	// Get user from database
	var user models.User
	result := h.db.Where("firebase_uid = ?", uid).First(&user)
	if result.Error == nil {
		// Update user's last_logout field (you would need to add this field to your User model)
		// This is optional, but can be useful for tracking user activity
		h.db.Model(&user).Updates(map[string]interface{}{
			"last_activity": time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetSession checks if the user has a valid session
func (h *AuthHandler) GetSession(c *gin.Context) {
	// Get Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// No token provided
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Extract token from Bearer header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	token := parts[1]

	// Verify the Firebase token
	uid, err := firebase.VerifyToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Get user from database
	var user models.User
	result := h.db.Where("firebase_uid = ?", uid).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Session is valid, return the user
	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"role":         user.Role,
		},
	})
}