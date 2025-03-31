package handlers

import (
	"net/http"
	"strconv"

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
	userID := c.GetUint("userID")

	var sessions []models.Session
	if err := h.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sessions"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

func (h *SessionHandler) RevokeSession(c *gin.Context) {
	sessionID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert sessionID to uint
	sessionIDUint, err := strconv.ParseUint(sessionID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	var session models.Session
	if err := h.db.First(&session, sessionIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Check if session belongs to user
	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to revoke this session"})
		return
	}

	// Deactivate session
	session.IsActive = false
	if err := h.db.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session revoked successfully"})
}