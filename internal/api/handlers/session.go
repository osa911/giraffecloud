package handlers

import (
	"context"
	"net/http"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	sessionRepo repository.SessionRepository
}

func NewSessionHandler(sessionRepo repository.SessionRepository) *SessionHandler {
	return &SessionHandler{sessionRepo: sessionRepo}
}

func (h *SessionHandler) GetSessions(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	sessions, err := h.sessionRepo.GetActiveSessions(context.Background(), uint32(userID))
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to get sessions")
		return
	}

	// Simplify sessions for response
	var sessionResponses []gin.H
	for _, s := range sessions {
		sessionResponses = append(sessionResponses, gin.H{
			"id":         s.ID,
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

	session, err := h.sessionRepo.GetUserSession(context.Background(), sessionID, uint32(userID))
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Session not found")
		return
	}

	// Mark session as inactive
	if err := h.sessionRepo.RevokeSession(context.Background(), session); err != nil {
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

	if err := h.sessionRepo.RevokeAllUserSessions(context.Background(), uint32(userID)); err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to revoke sessions")
		return
	}

	// Clear current cookies regardless
	c.SetCookie(constants.CookieSession, "", -1, constants.CookiePathRoot, "", true, true)
	c.SetCookie(constants.CookieAuthToken, "", -1, constants.CookiePathAPI, "", true, true)

	c.JSON(http.StatusOK, common.NewMessageResponse("All sessions successfully revoked"))
}