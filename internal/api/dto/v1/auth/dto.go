package auth

import (
	"time"

	"giraffecloud/internal/api/dto/v1/user"
)

// LoginRequest represents the login request payload
type LoginRequest struct {
	Token string `json:"token" binding:"required"`
}

// LoginResponse represents the response after a successful login
type LoginResponse struct {
	User user.UserResponse `json:"user"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Name        string `json:"name" binding:"required"`
	Token       string `json:"token" binding:"required"`
}

// RegisterResponse represents the response after a successful registration
type RegisterResponse struct {
	User user.UserResponse `json:"user"`
}

// SessionResponse represents session information
type SessionResponse struct {
	Token      string    `json:"token"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// SessionValidationResponse represents the response for session validation
type SessionValidationResponse struct {
	Valid bool               `json:"valid"`
	User  *user.UserResponse `json:"user,omitempty"`
}

// VerifyTokenRequest represents the request for verifying a Firebase ID token
type VerifyTokenRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}