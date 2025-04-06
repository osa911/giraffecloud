package auth

// LoginRequest represents the login request payload
type LoginRequest struct {
	Token string `json:"token" binding:"required"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Name        string `json:"name" binding:"required"`
	FirebaseUID string `json:"firebase_uid" binding:"required"`
}

// LoginResponse represents the response after a successful login
type LoginResponse struct {
	User UserResponse `json:"user"`
}

// RegisterResponse represents the response after a successful registration
type RegisterResponse struct {
	User UserResponse `json:"user"`
}

// SessionResponse represents the response for session validation
type SessionResponse struct {
	Valid bool         `json:"valid"`
	User  *UserResponse `json:"user,omitempty"`
}

// UserResponse contains minimal user information for auth responses
type UserResponse struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}