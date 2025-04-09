package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/user"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/models"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userModel, exists := c.Get(constants.ContextKeyUser)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusUnauthorized, common.ErrCodeUnauthorized, "User not found in context")
		return
	}

	u, ok := userModel.(models.User)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(&u)

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

	currentUser, ok := contextUser.(models.User)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid user type in context")
		return
	}

	// Get profile data from context (set by validation middleware)
	profileData, _ := c.Get(constants.ContextKeyUpdateProfile)
	if profileData == nil {
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
	var dbUser models.User
	if err := h.db.First(&dbUser, currentUser.ID).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Apply changes using mapper
	mapper.ApplyUpdateProfileRequest(&dbUser, profileUpdate)

	if err := h.db.Save(&dbUser).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update profile")
		return
	}

	// Return updated user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(&dbUser)))
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var userModel models.User
	if err := h.db.First(&userModel, userID).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	if err := h.db.Delete(&userModel).Error; err != nil {
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

	var users []models.User
	var totalCount int64

	// Get total count for pagination
	h.db.Model(&models.User{}).Count(&totalCount)

	// Get users with pagination
	if err := h.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
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
	userID := c.Param("id")

	// Fetch user from database
	var dbUser models.User
	if err := h.db.First(&dbUser, userID).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Return user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(&dbUser)))
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")

	// Fetch user from database
	var dbUser models.User
	if err := h.db.First(&dbUser, userID).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Get user update data from request
	var userUpdate user.UpdateUserRequest
	if err := c.ShouldBindJSON(&userUpdate); err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeValidation, "Invalid request data")
		return
	}

	// Apply changes using mapper
	mapper.ApplyUpdateUserRequest(&dbUser, &userUpdate)

	if err := h.db.Save(&dbUser).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update user")
		return
	}

	// Return updated user
	c.JSON(http.StatusOK, common.NewSuccessResponse(mapper.UserToUserResponse(&dbUser)))
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// Fetch user from database
	var dbUser models.User
	if err := h.db.First(&dbUser, userID).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "User not found")
		return
	}

	// Mark user as inactive instead of deleting
	if err := h.db.Model(&dbUser).Updates(map[string]interface{}{"is_active": false}).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to delete user")
		return
	}

	c.JSON(http.StatusOK, common.NewMessageResponse("User successfully deleted"))
}

