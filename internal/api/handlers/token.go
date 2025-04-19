package handlers

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/token"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"

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
		utils.HandleAPIError(c, err, common.ErrCodeBadRequest, "Invalid request body")
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Unauthorized")
		return
	}

	response, err := h.tokenService.CreateToken(c.Request.Context(), userID.(uint32), &req)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to create token")
		return
	}

	utils.HandleCreated(c, response)
}

func (h *TokenHandler) ListTokens(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		utils.HandleAPIError(c, nil, common.ErrCodeUnauthorized, "Unauthorized")
		return
	}

	tokens, err := h.tokenService.ListTokens(c.Request.Context(), userID.(uint32))
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to list tokens")
		return
	}

	utils.HandleSuccess(c, tokens)
}

func (h *TokenHandler) RevokeToken(c *gin.Context) {
	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeBadRequest, "Invalid token ID")
		return
	}

	if err := h.tokenService.RevokeToken(c.Request.Context(), tokenID); err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to revoke token")
		return
	}

	utils.HandleNoContent(c)
}