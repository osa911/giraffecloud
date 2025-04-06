package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/tunnel"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TunnelHandler struct {
	db *gorm.DB
}

func NewTunnelHandler(db *gorm.DB) *TunnelHandler {
	return &TunnelHandler{db: db}
}

func (h *TunnelHandler) ListTunnels(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var tunnels []models.Tunnel
	if err := h.db.Where("user_id = ?", userID).Find(&tunnels).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to fetch tunnels", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert domain models to DTOs and wrap in APIResponse
	tunnelResponses := mapper.TunnelsToTunnelResponses(tunnels)
	response := common.NewSuccessResponse(tunnel.ListTunnelsResponse{
		Tunnels:    tunnelResponses,
		TotalCount: int64(len(tunnelResponses)),
		Page:       1,      // Pagination not implemented yet
		PageSize:   100,    // Pagination not implemented yet
	})

	c.JSON(http.StatusOK, response)
}

func (h *TunnelHandler) CreateTunnel(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	// Get tunnel data from context
	tunnelData, exists := c.Get(constants.ContextKeyCreateTunnel)
	if !exists {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Tunnel data not found in context. Ensure validation middleware is applied.", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Extract tunnel data
	tunnelPtr, ok := tunnelData.(*tunnel.CreateTunnelRequest)
	if !ok {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid tunnel data format", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	tunnel := models.Tunnel{
		Name:       tunnelPtr.Name,
		UserID:     userID,
		Protocol:   models.Protocol(tunnelPtr.Protocol),
		LocalPort:  tunnelPtr.LocalPort,
		RemoteHost: tunnelPtr.RemoteHost,
		Status:     models.StatusInactive,
		IsEnabled:  true,
	}

	if err := h.db.Create(&tunnel).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to create tunnel", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert domain model to DTO and wrap in APIResponse
	tunnelResponse := mapper.TunnelToTunnelResponse(&tunnel)
	response := common.NewSuccessResponse(tunnelResponse)

	c.JSON(http.StatusCreated, response)
}

func (h *TunnelHandler) GetTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid tunnel ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Get tunnel
	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Tunnel not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if the tunnel belongs to the user
	if tunnel.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Access denied", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	// Convert domain model to DTO and wrap in APIResponse
	tunnelResponse := mapper.TunnelToTunnelResponse(&tunnel)
	response := common.NewSuccessResponse(tunnelResponse)

	c.JSON(http.StatusOK, response)
}

func (h *TunnelHandler) UpdateTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid tunnel ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Get tunnel update data from context
	tunnelData, exists := c.Get(constants.ContextKeyUpdateTunnel)
	if !exists {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Tunnel update data not found in context. Ensure validation middleware is applied.", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Extract tunnel data
	tunnelPtr, ok := tunnelData.(*tunnel.UpdateTunnelRequest)
	if !ok {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Invalid tunnel update data format", nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Tunnel not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if the tunnel belongs to the user
	if tunnel.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Access denied", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	// Update fields
	if tunnelPtr.Name != "" {
		tunnel.Name = tunnelPtr.Name
	}
	if tunnelPtr.Protocol != "" {
		tunnel.Protocol = models.Protocol(tunnelPtr.Protocol)
	}
	if tunnelPtr.LocalPort > 0 {
		tunnel.LocalPort = tunnelPtr.LocalPort
	}
	if tunnelPtr.RemoteHost != "" {
		tunnel.RemoteHost = tunnelPtr.RemoteHost
	}
	tunnel.IsEnabled = tunnelPtr.IsEnabled

	if err := h.db.Save(&tunnel).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to update tunnel", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert domain model to DTO and wrap in APIResponse
	tunnelResponse := mapper.TunnelToTunnelResponse(&tunnel)
	response := common.NewSuccessResponse(tunnelResponse)

	c.JSON(http.StatusOK, response)
}

func (h *TunnelHandler) DeleteTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid tunnel ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Tunnel not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if the tunnel belongs to the user
	if tunnel.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Access denied", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	if err := h.db.Delete(&tunnel).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to delete tunnel", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Return success response
	response := common.NewMessageResponse("Tunnel deleted successfully")
	c.JSON(http.StatusOK, response)
}

func (h *TunnelHandler) StartTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid tunnel ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Tunnel not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if the tunnel belongs to the user
	if tunnel.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Access denied", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	// TODO: Implement tunnel start logic
	tunnel.Status = models.StatusStarting
	tunnel.IsEnabled = true

	if err := h.db.Save(&tunnel).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to start tunnel", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Return success response
	response := common.NewMessageResponse("Tunnel started successfully")
	c.JSON(http.StatusOK, response)
}

func (h *TunnelHandler) StopTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		response := common.NewErrorResponse(common.ErrCodeBadRequest, "Invalid tunnel ID", nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeNotFound, "Tunnel not found", nil)
		c.JSON(http.StatusNotFound, response)
		return
	}

	// Check if the tunnel belongs to the user
	if tunnel.UserID != userID {
		response := common.NewErrorResponse(common.ErrCodeForbidden, "Access denied", nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	// TODO: Implement tunnel stop logic
	tunnel.Status = models.StatusStopping
	tunnel.IsEnabled = false

	if err := h.db.Save(&tunnel).Error; err != nil {
		response := common.NewErrorResponse(common.ErrCodeInternalServer, "Failed to stop tunnel", err)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Return success response
	response := common.NewMessageResponse("Tunnel stopped successfully")
	c.JSON(http.StatusOK, response)
}