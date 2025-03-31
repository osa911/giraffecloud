package handlers

import (
	"net/http"
	"time"

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
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	Name         string `json:"name" binding:"required"`
	Organization string `json:"organization"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// TODO: Verify password hash
	// if !user.VerifyPassword(req.Password) {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	// 	return
	// }

	// Create session
	session := models.Session{
		UserID:    user.ID,
		Token:     generateToken(), // TODO: Implement token generation
		DeviceName: c.GetHeader("User-Agent"),
		DeviceID:   c.GetHeader("X-Device-ID"),
		IPAddress:  c.ClientIP(),
		LastUsed:   time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IsActive:   true,
	}

	if err := h.db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Update user last login
	user.LastLogin = time.Now()
	h.db.Save(&user)

	c.JSON(http.StatusOK, gin.H{
		"token": session.Token,
		"user": gin.H{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"organization": user.Organization,
			"role":         user.Role,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user exists
	var existingUser models.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Create user
	user := models.User{
		Email:        req.Email,
		PasswordHash: req.Password, // TODO: Hash password
		Name:         req.Name,
		Organization: req.Organization,
		Role:         models.RoleUser,
		IsActive:     true,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Create session
	session := models.Session{
		UserID:    user.ID,
		Token:     generateToken(), // TODO: Implement token generation
		DeviceName: c.GetHeader("User-Agent"),
		DeviceID:   c.GetHeader("X-Device-ID"),
		IPAddress:  c.ClientIP(),
		LastUsed:   time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IsActive:   true,
	}

	if err := h.db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": session.Token,
		"user": gin.H{
			"id":           user.ID,
			"email":        user.Email,
			"name":         user.Name,
			"organization": user.Organization,
			"role":         user.Role,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}

	// Remove "Bearer " prefix
	token = token[7:]

	// Deactivate session
	if err := h.db.Model(&models.Session{}).Where("token = ?", token).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// TODO: Implement token generation
func generateToken() string {
	return "dummy-token" // Replace with proper token generation
}