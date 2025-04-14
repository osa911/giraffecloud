package handlers

import (
	"context"
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/user"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	repository repository.UserRepository
}

func NewUserHandler(repository repository.UserRepository) *UserHandler {
	return &UserHandler{
		repository: repository,
	}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userModel, exists := c.Get(constants.ContextKeyUser)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusUnauthorized, common.ErrCodeUnauthorized, "User not found in context")
		return
	}

	u, ok := userModel.(*ent.User)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(u)

	// Use proper DTO format with wrapped response
	c.JSON(http.StatusOK, common.NewSuccessResponse(userResponse))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	// Get user from context (set by auth middleware)
	contextUser, exists := c.Get(constants.ContextKeyUser)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusUnauthorized, common.ErrCodeUnauthorized, "User not found in context")
		return
	}

	currentUser, ok := contextUser.(*ent.User)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	// Get profile data from context (set by validation middleware)
	profileData, exists := c.Get(constants.ContextKeyUpdateProfile)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusBadRequest, common.ErrCodeBadRequest, "Missing profile data")
		return
	}

	// Extract and validate profile data
	profileUpdate, ok := profileData.(*user.UpdateProfileRequest)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid profile data format")
		return
	}

	// Fetch user from database to ensure we have the latest data
	dbUser, err := h.repository.Get(context.Background(), currentUser.ID)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Create update query
	update := dbUser.Update()
	if profileUpdate.Name != "" {
		update.SetName(profileUpdate.Name)
	}

	// Apply update
	updatedUser, err := h.repository.Update(context.Background(), dbUser.ID, update)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update profile")
		return
	}

	// Return updated user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(updatedUser)))
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID := uint32(c.GetUint(constants.ContextKeyUserID))

	if err := h.repository.Delete(context.Background(), userID); err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to delete user")
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
		"message": "User deleted successfully",
	}))
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	// Ensure reasonable defaults
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	// Get total count
	totalCount, err := h.repository.Count(context.Background())
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to get total count")
		return
	}

	// Get users with pagination
	users, err := h.repository.List(context.Background(), offset, pageSize)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to fetch users")
		return
	}

	// Convert to response DTOs using mapper
	userResponses := mapper.UsersToUserResponses(users)

	response := user.ListUsersResponse{
		Users:      userResponses,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(response))
}

func (h *UserHandler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeValidation, "Invalid user ID")
		return
	}

	// Fetch user from database
	dbUser, err := h.repository.Get(context.Background(), uint32(userID))
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Return user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(dbUser)))
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeValidation, "Invalid user ID")
		return
	}

	// Fetch user from database
	dbUser, err := h.repository.Get(context.Background(), uint32(userID))
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Get user update data from request
	var userUpdate user.UpdateUserRequest
	if err := c.ShouldBindJSON(&userUpdate); err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeValidation, "Invalid request data")
		return
	}

	// Create update query
	update := dbUser.Update()
	if userUpdate.Name != "" {
		update.SetName(userUpdate.Name)
	}
	if userUpdate.Email != "" {
		update.SetEmail(userUpdate.Email)
	}
	update.SetIsActive(userUpdate.IsActive)

	// Apply update
	updatedUser, err := h.repository.Update(context.Background(), dbUser.ID, update)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user")
		return
	}

	// Return updated user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(updatedUser)))
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeValidation, "Invalid user ID")
		return
	}

	if err := h.repository.Delete(context.Background(), uint32(userID)); err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to delete user")
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
		"message": "User deleted successfully",
	}))
}

