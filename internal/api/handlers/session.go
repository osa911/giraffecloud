package handlers

import (
	"net/http"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/models"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SessionHandler struct {
	db *gorm.DB
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

func (h *SessionHandler) GetSessions(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var sessions []models.Session
	if err := h.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&sessions).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to get sessions")
		return
	}

	// Simplify sessions for response
	var sessionResponses []gin.H
	for _, s := range sessions {
		sessionResponses = append(sessionResponses, gin.H{
			"id":         s.ID,
			"deviceName": s.DeviceName,
			"lastUsed":   s.LastUsed,
			"ipAddress":  s.IPAddress,
			"createdAt":  s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{"sessions": sessionResponses}))
}

func (h *SessionHandler) RevokeSession(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)
	sessionID := c.Param("id")

	var session models.Session
	if err := h.db.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Session not found")
		return
	}

	// Mark session as inactive
	session.IsActive = false
	if err := h.db.Save(&session).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to revoke session")
		return
	}

	// If we're revoking the current session, also clear cookies
	if sessionToken, err := c.Cookie(constants.CookieAuthToken); err == nil && sessionToken == session.Token {
		c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, "", true, true)
		c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathAPI, "", true, true)
	}

	c.JSON(http.StatusOK, common.NewMessageResponse("Session successfully revoked"))
}

func (h *SessionHandler) RevokeAllSessions(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	if err := h.db.Model(&models.Session{}).Where("user_id = ?", userID).Updates(map[string]interface{}{"is_active": false}).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to revoke sessions")
		return
	}

	// Clear current cookies regardless
	c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, "", true, true)
	c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathAPI, "", true, true)

	c.JSON(http.StatusOK, common.NewMessageResponse("All sessions successfully revoked"))
}