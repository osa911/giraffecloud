package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/user"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/models"

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
		c.JSON(http.StatusUnauthorized, common.NewErrorResponse(common.ErrCodeUnauthorized, "User not found in context", nil))
		return
	}

	u, ok := userModel.(models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid user type in context", nil))
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(&u)

	// Use proper DTO format with wrapped response
	c.JSON(http.StatusOK, common.NewSuccessResponse(userResponse))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	// Get validated profile from context instead of reading request body again
	profileData, exists := c.Get(constants.ContextKeyUpdateProfile)
	if !exists {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.ErrCodeBadRequest, "Missing profile data", nil))
		return
	}

	// Cast to pointer type since that's what the validation middleware stores
	profilePtr, ok := profileData.(*user.UpdateProfileRequest)
	if !ok {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid profile data format", nil))
		return
	}

	var userModel models.User
	if err := h.db.First(&userModel, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil))
		return
	}

	// Apply changes using mapper
	mapper.ApplyUpdateProfileRequest(&userModel, profilePtr)

	if err := h.db.Save(&userModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update profile", err))
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(&userModel)
	c.JSON(http.StatusOK, common.NewSuccessResponse(userResponse))
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var userModel models.User
	if err := h.db.First(&userModel, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil))
		return
	}

	if err := h.db.Delete(&userModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to delete user", err))
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
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to fetch users", err))
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
	id := c.Param("id")

	var userModel models.User
	if err := h.db.First(&userModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil))
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(&userModel)
	c.JSON(http.StatusOK, common.NewSuccessResponse(userResponse))
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var userModel models.User
	if err := h.db.First(&userModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil))
		return
	}

	var updateRequest user.UpdateUserRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.ErrCodeValidation, "Invalid request data", err))
		return
	}

	// Apply changes using mapper
	mapper.ApplyUpdateUserRequest(&userModel, &updateRequest)

	if err := h.db.Save(&userModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update user", err))
		return
	}

	// Convert to response DTO using mapper
	userResponse := mapper.UserToUserResponse(&userModel)
	c.JSON(http.StatusOK, common.NewSuccessResponse(userResponse))
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	var userModel models.User
	if err := h.db.First(&userModel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil))
		return
	}

	if err := h.db.Delete(&userModel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to delete user", err))
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
		"message": "User deleted successfully",
	}))
}

