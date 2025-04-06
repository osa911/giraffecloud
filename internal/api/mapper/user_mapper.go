package mapper

import (
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/api/dto/v1/user"
	"giraffecloud/internal/models"
)

// UserToUserResponse converts a domain User model to a UserResponse DTO
func UserToUserResponse(u *models.User) *user.UserResponse {
	if u == nil {
		return nil
	}

	return &user.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      string(u.Role),
		IsActive:  u.IsActive,
		LastLogin: u.LastLogin,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// UsersToUserResponses converts a slice of domain User models to UserResponse DTOs
func UsersToUserResponses(users []models.User) []user.UserResponse {
	result := make([]user.UserResponse, len(users))
	for i, u := range users {
		user := u // Create a copy to avoid issues with references in the loop
		result[i] = *UserToUserResponse(&user)
	}
	return result
}

// UserToAuthUserResponse converts a domain User model to an auth UserResponse DTO
func UserToAuthUserResponse(u *models.User) *auth.UserResponse {
	if u == nil {
		return nil
	}

	return &auth.UserResponse{
		ID:    u.ID,
		Email: u.Email,
		Name:  u.Name,
		Role:  string(u.Role),
	}
}

// ApplyUpdateProfileRequest applies changes from UpdateProfileRequest to a User model
func ApplyUpdateProfileRequest(u *models.User, req *user.UpdateProfileRequest) {
	if req.Name != "" {
		u.Name = req.Name
	}
}

// ApplyUpdateUserRequest applies changes from UpdateUserRequest to a User model
func ApplyUpdateUserRequest(u *models.User, req *user.UpdateUserRequest) {
	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Role != "" {
		u.Role = models.Role(req.Role)
	}
	u.IsActive = req.IsActive
}