package middleware

import (
	"encoding/json"
	"net/http"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/dto/v1/tunnel"
	"giraffecloud/internal/api/dto/v1/user"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationMiddleware handles request validation
type ValidationMiddleware struct {
	validator *validator.Validate
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validator.New(),
	}
}

// validateRequest is a helper function to validate a request against a struct
func (m *ValidationMiddleware) validateRequest(c *gin.Context, obj interface{}, contextKey string) bool {
	// Get raw body from context
	rawBody, exists := c.Get(constants.ContextKeyRawBody)
	if !exists {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(
			common.ErrCodeInternalServer,
			"Request body not found in context. Ensure body reader middleware is applied.",
			nil,
		))
		c.Abort()
		return false
	}

	// Use the raw body bytes
	bodyBytes, ok := rawBody.([]byte)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(
			common.ErrCodeInternalServer,
			"Invalid body format in context",
			nil,
		))
		c.Abort()
		return false
	}

	// If body is empty and we need to validate a struct, return validation error
	if len(bodyBytes) == 0 {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.ErrCodeValidation,
			"Request body is empty",
			nil,
		))
		c.Abort()
		return false
	}

	// Unmarshal JSON
	if err := json.Unmarshal(bodyBytes, obj); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.ErrCodeValidation,
			"Invalid JSON format",
			err,
		))
		c.Abort()
		return false
	}

	// Validate
	if err := m.validator.Struct(obj); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.ErrCodeValidation,
			"Validation failed",
			err,
		))
		c.Abort()
		return false
	}

	// Set validated object in context
	c.Set(contextKey, obj)
	return true
}

// ValidateLoginRequest validates login request
func (m *ValidationMiddleware) ValidateLoginRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginReq auth.LoginRequest
		if m.validateRequest(c, &loginReq, constants.ContextKeyLogin) {
			c.Next()
		}
	}
}

// ValidateRegisterRequest validates registration request
func (m *ValidationMiddleware) ValidateRegisterRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var regReq auth.RegisterRequest
		if m.validateRequest(c, &regReq, constants.ContextKeyRegister) {
			c.Next()
		}
	}
}

// ValidateUpdateProfileRequest validates profile update request
func (m *ValidationMiddleware) ValidateUpdateProfileRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var profileReq user.UpdateProfileRequest
		if m.validateRequest(c, &profileReq, constants.ContextKeyUpdateProfile) {
			c.Next()
		}
	}
}

// ValidateUpdateUserRequest validates user update request (admin)
func (m *ValidationMiddleware) ValidateUpdateUserRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userReq user.UpdateUserRequest
		if m.validateRequest(c, &userReq, constants.ContextKeyUpdateUser) {
			c.Next()
		}
	}
}

// ValidateCreateTunnelRequest validates tunnel creation request
func (m *ValidationMiddleware) ValidateCreateTunnelRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tunnelReq tunnel.CreateTunnelRequest
		if m.validateRequest(c, &tunnelReq, constants.ContextKeyCreateTunnel) {
			c.Next()
		}
	}
}

// ValidateUpdateTunnelRequest validates tunnel update request
func (m *ValidationMiddleware) ValidateUpdateTunnelRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tunnelReq tunnel.UpdateTunnelRequest
		if m.validateRequest(c, &tunnelReq, constants.ContextKeyUpdateTunnel) {
			c.Next()
		}
	}
}

// ValidateVerifyTokenRequest validates token verification request
func (m *ValidationMiddleware) ValidateVerifyTokenRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var verifyReq auth.VerifyTokenRequest
		if m.validateRequest(c, &verifyReq, constants.ContextKeyVerifyToken) {
			c.Next()
		}
	}
}