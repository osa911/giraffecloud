package middleware

import (
	"giraffecloud/internal/api/validation"
	"io"
	"net/http"

	"bytes"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationMiddleware handles request validation
type ValidationMiddleware struct {
	validate *validator.Validate
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	validate := validator.New()
	validation.RegisterValidators(validate)
	return &ValidationMiddleware{
		validate: validate,
	}
}

// ValidateLoginRequest validates login request
func (m *ValidationMiddleware) ValidateLoginRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read raw body first
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error reading request body",
			})
			c.Abort()
			return
		}

		// Restore body for binding
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var login struct {
			Token string `json:"token" binding:"required"`
		}

		if err := c.ShouldBindJSON(&login); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
				"errors": validation.FormatValidationError(err),
				"received": string(bodyBytes),
			})
			c.Abort()
			return
		}

		// Restore body again for the handler
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		c.Set("login", login)
		c.Next()
	}
}

// ValidateRegisterRequest validates registration request
func (m *ValidationMiddleware) ValidateRegisterRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read raw body first
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error reading request body",
			})
			c.Abort()
			return
		}

		// Restore body for binding
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var register struct {
			Email        string `json:"email" binding:"required,email"`
			Name         string `json:"name" binding:"required"`
			FirebaseUID  string `json:"firebase_uid" binding:"required"`
		}

		if err := c.ShouldBindJSON(&register); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
				"errors": validation.FormatValidationError(err),
			})
			c.Abort()
			return
		}

		// Restore body again for the handler
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		c.Set("register", register)
		c.Next()
	}
}

// ValidateUpdateProfileRequest validates profile update request
func (m *ValidationMiddleware) ValidateUpdateProfileRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var profile struct {
			Name         string `json:"name" binding:"omitempty,name"`
			Website      string `json:"website" binding:"omitempty,url"`
		}

		if err := c.ShouldBindJSON(&profile); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
				"errors": validation.FormatValidationError(err),
			})
			c.Abort()
			return
		}

		c.Set("profile", profile)
		c.Next()
	}
}