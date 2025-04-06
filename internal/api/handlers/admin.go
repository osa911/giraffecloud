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

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Find(&users).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to fetch users", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert domain models to DTOs and wrap in APIResponse
	userResponses := mapper.UsersToUserResponses(users)
	response := common.NewSuccessResponse(user.ListUsersResponse{
		Users:      userResponses,
		TotalCount: int64(len(userResponses)),
		Page:       1,  // Pagination not implemented yet
		PageSize:   100, // Pagination not implemented yet
	})

	c.JSON(http.StatusOK, response)
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	// Convert userID to uint
	userIDUint, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid user ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var user models.User
	if err := h.db.First(&user, userIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Convert domain model to DTO and wrap in APIResponse
	userResponse := mapper.UserToUserResponse(&user)
	response := common.NewSuccessResponse(userResponse)

	c.JSON(http.StatusOK, response)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")

	// Convert userID to uint
	userIDUint, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid user ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Get validated user data from context
	userData, exists := c.Get(constants.ContextKeyUpdateUser)
	if !exists {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "User update data not found in context. Ensure validation middleware is applied.", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Extract user data
	userPtr, ok := userData.(*user.UpdateUserRequest)
	if !ok {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid user data format", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	var user models.User
	if err := h.db.First(&user, userIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Apply the updates
	mapper.ApplyUpdateUserRequest(&user, userPtr)

	if err := h.db.Save(&user).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update user", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert domain model to DTO and wrap in APIResponse
	userResponse := mapper.UserToUserResponse(&user)
	response := common.NewSuccessResponse(userResponse)

	c.JSON(http.StatusOK, response)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	// Convert userID to uint
	userIDUint, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid user ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var user models.User
	if err := h.db.First(&user, userIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "User not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if user is the last admin
	if user.Role == models.RoleAdmin {
		var adminCount int64
		if err := h.db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount).Error; err != nil {
			response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to check admin count", nil)
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		if adminCount == 1 {
			response := common.NewErrorResponse(common.ErrCodeBadRequest, "Cannot delete the last admin user", nil)
			c.JSON(http.StatusBadRequest, response)
			return
		}
	}

	// Delete user's sessions
	if err := h.db.Where("user_id = ?", userIDUint).Delete(&models.Session{}).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to delete user sessions", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Delete user
	if err := h.db.Delete(&user).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to delete user", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Return success response
	response := common.NewMessageResponse("User deleted successfully")
	c.JSON(http.StatusOK, response)
}