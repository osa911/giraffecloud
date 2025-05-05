package handlers

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"
	"strconv"

	"giraffecloud/internal/logging"

	"github.com/gin-gonic/gin"
)

// TunnelHandler handles tunnel-related HTTP requests
type TunnelHandler struct {
	tunnelService service.TunnelService
}

// NewTunnelHandler creates a new tunnel handler instance
func NewTunnelHandler(tunnelService service.TunnelService) *TunnelHandler {
	return &TunnelHandler{
		tunnelService: tunnelService,
	}
}

// CreateTunnel creates a new tunnel
func (h *TunnelHandler) CreateTunnel(c *gin.Context) {
	var req struct {
		Domain     string `json:"domain" binding:"required"`
		TargetPort int    `json:"target_port" binding:"required,min=1,max=65535"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logging.GetGlobalLogger().Error("CreateTunnel: Invalid request data: %+v, error: %v", req, err)
		utils.HandleAPIError(c, err, common.ErrCodeValidation, "Invalid request data")
		return
	}

	userID := c.MustGet(constants.ContextKeyUserID).(uint32)
	tunnel, err := h.tunnelService.CreateTunnel(c.Request.Context(), userID, req.Domain, req.TargetPort)
	if err != nil {
		logging.GetGlobalLogger().Error("CreateTunnel: Failed to create tunnel for userID=%d, req=%+v, error: %v", userID, req, err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to create tunnel")
		return
	}

	utils.HandleCreated(c, tunnel)
}

// ListTunnels lists all tunnels for the authenticated user
func (h *TunnelHandler) ListTunnels(c *gin.Context) {
	userID := c.MustGet(constants.ContextKeyUserID).(uint32)
	tunnels, err := h.tunnelService.ListTunnels(c.Request.Context(), userID)
	if err != nil {
		logging.GetGlobalLogger().Error("ListTunnels: Failed to list tunnels for userID=%d, error: %v", userID, err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to list tunnels")
		return
	}

	utils.HandleSuccess(c, tunnels)
}

// GetTunnel gets a specific tunnel by ID
func (h *TunnelHandler) GetTunnel(c *gin.Context) {
	userID := c.MustGet(constants.ContextKeyUserID).(uint32)
	tunnelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logging.GetGlobalLogger().Error("GetTunnel: Invalid tunnel ID: %v, error: %v", c.Param("id"), err)
		utils.HandleAPIError(c, err, common.ErrCodeValidation, "Invalid tunnel ID")
		return
	}

	tunnel, err := h.tunnelService.GetTunnel(c.Request.Context(), userID, uint32(tunnelID))
	if err != nil {
		logging.GetGlobalLogger().Error("GetTunnel: Tunnel not found for userID=%d, tunnelID=%d, error: %v", userID, tunnelID, err)
		utils.HandleAPIError(c, err, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	utils.HandleSuccess(c, tunnel)
}

// DeleteTunnel deletes a tunnel
func (h *TunnelHandler) DeleteTunnel(c *gin.Context) {
	userID := c.MustGet(constants.ContextKeyUserID).(uint32)
	tunnelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logging.GetGlobalLogger().Error("DeleteTunnel: Invalid tunnel ID: %v, error: %v", c.Param("id"), err)
		utils.HandleAPIError(c, err, common.ErrCodeValidation, "Invalid tunnel ID")
		return
	}

	if err := h.tunnelService.DeleteTunnel(c.Request.Context(), userID, uint32(tunnelID)); err != nil {
		logging.GetGlobalLogger().Error("DeleteTunnel: Failed to delete tunnel for userID=%d, tunnelID=%d, error: %v", userID, tunnelID, err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to delete tunnel")
		return
	}

	utils.HandleNoContent(c)
}

// UpdateTunnel updates a tunnel's configuration
func (h *TunnelHandler) UpdateTunnel(c *gin.Context) {
	var req struct {
		IsActive   *bool  `json:"is_active,omitempty"`
		TargetPort *int   `json:"target_port,omitempty" binding:"omitempty,min=1,max=65535"`
		Domain     string `json:"domain,omitempty" binding:"omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logging.GetGlobalLogger().Error("UpdateTunnel: Invalid request data: %+v, error: %v", req, err)
		utils.HandleAPIError(c, err, common.ErrCodeValidation, "Invalid request data")
		return
	}

	userID := c.MustGet(constants.ContextKeyUserID).(uint32)
	tunnelID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logging.GetGlobalLogger().Error("UpdateTunnel: Invalid tunnel ID: %v, error: %v", c.Param("id"), err)
		utils.HandleAPIError(c, err, common.ErrCodeValidation, "Invalid tunnel ID")
		return
	}

	tunnel, err := h.tunnelService.UpdateTunnel(c.Request.Context(), userID, uint32(tunnelID), &req)
	if err != nil {
		logging.GetGlobalLogger().Error("UpdateTunnel: Failed to update tunnel for userID=%d, tunnelID=%d, req=%+v, error: %v", userID, tunnelID, req, err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to update tunnel")
		return
	}

	utils.HandleSuccess(c, tunnel)
}