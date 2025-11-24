package mapper

import (
	"time"

	userDto "github.com/osa911/giraffecloud/internal/api/dto/v1/user"
	"github.com/osa911/giraffecloud/internal/db/ent"
)

// UserToUserResponse converts an Ent User entity to a UserResponse DTO
func UserToUserResponse(u *ent.User) *userDto.UserResponse {
	if u == nil {
		return nil
	}

	var lastLoginStr string
	if u.LastLogin != nil {
		lastLoginStr = u.LastLogin.Format(time.RFC3339)
	}

	var lastLoginIP string
	if u.LastLoginIP != nil {
		lastLoginIP = *u.LastLoginIP
	}

	var lastActivityStr string
	if u.LastActivity != nil {
		lastActivityStr = u.LastActivity.Format(time.RFC3339)
	}

	return &userDto.UserResponse{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		Role:         u.Role,
		IsActive:     u.IsActive,
		LastLogin:    lastLoginStr,
		LastLoginIP:  lastLoginIP,
		LastActivity: lastActivityStr,
		CreatedAt:    u.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    u.UpdatedAt.Format(time.RFC3339),
	}
}

// UsersToUserResponses converts a slice of Ent User entities to UserResponse DTOs
func UsersToUserResponses(users []*ent.User) []*userDto.UserResponse {
	result := make([]*userDto.UserResponse, len(users))
	for i, u := range users {
		result[i] = UserToUserResponse(u)
	}
	return result
}

// ApplyUpdateUserRequest applies changes from UpdateUserRequest to a User update builder
func ApplyUpdateUserRequest(update *ent.UserUpdateOne, req *userDto.UpdateUserRequest) {
	if req.Name != "" {
		update.SetName(req.Name)
	}
	if req.Email != "" {
		update.SetEmail(req.Email)
	}
	update.SetIsActive(req.IsActive)
}
