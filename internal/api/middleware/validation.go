package middleware

import (
	"net/http"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/dto/v1/user"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidationMiddleware handles request validation
type ValidationMiddleware struct {
	validator *validator.Validate
}

// NewValidationMiddleware creates a new ValidationMiddleware instance
func NewValidationMiddleware() *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validator.New(),
	}
}

// validateRequest is a helper function to validate request data
func (m *ValidationMiddleware) validateRequest(c *gin.Context, data interface{}, contextKey string) bool {
	if err := c.ShouldBindJSON(data); err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid request data", err.Error())
		c.JSON(http.StatusBadRequest, response)
		c.Abort()
		return false
	}

	if err := m.validator.Struct(data); err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Validation failed", err.Error())
		c.JSON(http.StatusBadRequest, response)
		c.Abort()
		return false
	}

	c.Set(contextKey, data)
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
		var registerReq auth.RegisterRequest
		if m.validateRequest(c, &registerReq, constants.ContextKeyRegister) {
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

// ValidateUpdateUserRequest validates user update request
func (m *ValidationMiddleware) ValidateUpdateUserRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var updateReq user.UpdateUserRequest
		if m.validateRequest(c, &updateReq, constants.ContextKeyUpdateUser) {
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