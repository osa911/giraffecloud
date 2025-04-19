package handlers

import (
	"net/http"

	"giraffecloud/internal/api/dto/v1/token"
	"giraffecloud/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TokenHandler struct {
	tokenService *service.TokenService
}

func NewTokenHandler(tokenService *service.TokenService) *TokenHandler {
	return &TokenHandler{
		tokenService: tokenService,
	}
}

func (h *TokenHandler) CreateToken(c *gin.Context) {
	var req token.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get user ID from context (assuming it's set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	response, err := h.tokenService.CreateToken(c.Request.Context(), userID.(uint32), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *TokenHandler) ListTokens(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tokens, err := h.tokenService.ListTokens(c.Request.Context(), userID.(uint32))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *TokenHandler) RevokeToken(c *gin.Context) {
	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID"})
		return
	}

	if err := h.tokenService.RevokeToken(c.Request.Context(), tokenID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}