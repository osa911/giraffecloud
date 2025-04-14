package user

// UpdateProfileRequest represents the payload for updating a user's profile
type UpdateProfileRequest struct {
	Name string `json:"name" binding:"omitempty,min=2,max=50"`
}

// UpdateUserRequest represents the payload for updating a user's own profile
type UpdateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	IsActive bool   `json:"is_active"`
}

// UserResponse represents the user data returned in API responses
type UserResponse struct {
	ID           uint32    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	LastLogin    string    `json:"last_login"`
	LastLoginIP  string    `json:"last_login_ip"`
	LastActivity string    `json:"last_activity"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// ListUsersResponse represents the response for listing users
type ListUsersResponse struct {
	Users      []*UserResponse `json:"users"`
	TotalCount int64          `json:"total_count"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}