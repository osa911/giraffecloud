package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SessionHandler struct {
	db *gorm.DB
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

func (h *SessionHandler) ListSessions(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var sessions []models.Session
	if err := h.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&sessions).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to fetch sessions", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Return as a proper response
	response := common.NewSuccessResponse(gin.H{"sessions": sessions})
	c.JSON(http.StatusOK, response)
}

func (h *SessionHandler) RevokeSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert sessionID to uint
	sessionIDUint, err := strconv.ParseUint(sessionID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid session ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var session models.Session
	if err := h.db.First(&session, sessionIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Session not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if session belongs to user
	if session.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Not authorized to revoke this session", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	// Deactivate session
	session.IsActive = false
	if err := h.db.Save(&session).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to revoke session", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response := common.NewMessageResponse("Session revoked successfully")
	c.JSON(http.StatusOK, response)
}