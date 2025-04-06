package user

import "time"

// UpdateProfileRequest represents the payload for updating a user's profile
type UpdateProfileRequest struct {
	Name string `json:"name" binding:"omitempty,min=2,max=50"`
}

// UpdateUserRequest represents the payload for the admin to update a user
type UpdateUserRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive bool   `json:"isActive"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"isActive"`
	LastLogin time.Time `json:"lastLogin,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ListUsersResponse represents a paginated list of users
type ListUsersResponse struct {
	Users      []UserResponse `json:"users"`
	TotalCount int64          `json:"totalCount"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
}