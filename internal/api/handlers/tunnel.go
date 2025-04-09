package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/api/dto/v1/tunnel"
	"giraffecloud/internal/api/mapper"
	"giraffecloud/internal/models"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TunnelHandler struct {
	db *gorm.DB
}

func NewTunnelHandler(db *gorm.DB) *TunnelHandler {
	return &TunnelHandler{db: db}
}

func (h *TunnelHandler) GetTunnels(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	var tunnels []models.Tunnel
	if err := h.db.Where("user_id = ?", userID).Find(&tunnels).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to fetch tunnels")
		return
	}

	// Use mapper to convert to DTO
	tunnelResponses := mapper.TunnelsToTunnelResponses(tunnels)

	// Return as a proper response
	c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{"tunnels": tunnelResponses}))
}

func (h *TunnelHandler) CreateTunnel(c *gin.Context) {
	userID := c.GetUint(constants.ContextKeyUserID)

	// Get tunnel data from context (set by validation middleware)
	tunnelData, exists := c.Get(constants.ContextKeyCreateTunnel)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Tunnel data not found in context. Ensure validation middleware is applied.")
		return
	}

	// Extract tunnel data
	tunnelPtr, ok := tunnelData.(*tunnel.CreateTunnelRequest)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid tunnel data format")
		return
	}

	// Create a new tunnel
	newTunnel := models.Tunnel{
		UserID:    userID,
		Status:    models.StatusInactive,
		IsEnabled: true,
	}

	// Apply create tunnel request data using mapper
	mapper.ApplyCreateTunnelRequest(&newTunnel, tunnelPtr)

	if err := h.db.Create(&newTunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to create tunnel")
		return
	}

	// Convert to response DTO
	tunnelResponse := mapper.TunnelToTunnelResponse(&newTunnel)

	// Return with proper response format
	c.JSON(http.StatusCreated, common.NewSuccessResponse(tunnelResponse))
}

func (h *TunnelHandler) GetTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeBadRequest, "Invalid tunnel ID")
		return
	}

	var tunnel models.Tunnel
	if err := h.db.Where("id = ? AND user_id = ?", tunnelIDUint, userID).First(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	// Convert to response DTO
	tunnelResponse := mapper.TunnelToTunnelResponse(&tunnel)

	// Return with proper response format
	c.JSON(http.StatusOK, common.NewSuccessResponse(tunnelResponse))
}

func (h *TunnelHandler) UpdateTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeBadRequest, "Invalid tunnel ID")
		return
	}

	// Get tunnel update data from context
	tunnelData, exists := c.Get(constants.ContextKeyUpdateTunnel)
	if !exists {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Tunnel update data not found in context. Ensure validation middleware is applied.")
		return
	}

	// Extract tunnel update data
	tunnelPtr, ok := tunnelData.(*tunnel.UpdateTunnelRequest)
	if !ok {
		utils.HandleAPIError(c, nil, http.StatusInternalServerError, common.ErrCodeInternalServer, "Invalid tunnel update data format")
		return
	}

	// Fetch tunnel and check ownership
	var tunnel models.Tunnel
	if err := h.db.Where("id = ? AND user_id = ?", tunnelIDUint, userID).First(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	// Apply update tunnel request data using mapper
	mapper.ApplyUpdateTunnelRequest(&tunnel, tunnelPtr)

	// Save the updated tunnel
	if err := h.db.Save(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to update tunnel")
		return
	}

	// Convert to response DTO
	tunnelResponse := mapper.TunnelToTunnelResponse(&tunnel)

	// Return with proper response format
	c.JSON(http.StatusOK, common.NewSuccessResponse(tunnelResponse))
}

func (h *TunnelHandler) DeleteTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeBadRequest, "Invalid tunnel ID")
		return
	}

	// Fetch tunnel and check ownership
	var tunnel models.Tunnel
	if err := h.db.Where("id = ? AND user_id = ?", tunnelIDUint, userID).First(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	// Delete tunnel
	if err := h.db.Delete(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to delete tunnel")
		return
	}

	// Return success response
	c.JSON(http.StatusOK, common.NewMessageResponse("Tunnel deleted successfully"))
}

func (h *TunnelHandler) StartTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeBadRequest, "Invalid tunnel ID")
		return
	}

	// Fetch tunnel and check ownership
	var tunnel models.Tunnel
	if err := h.db.Where("id = ? AND user_id = ?", tunnelIDUint, userID).First(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	// TODO: Implement tunnel start logic
	tunnel.Status = models.StatusStarting
	tunnel.IsEnabled = true

	if err := h.db.Save(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to start tunnel")
		return
	}

	// Return success response
	c.JSON(http.StatusOK, common.NewMessageResponse("Tunnel started successfully"))
}

func (h *TunnelHandler) StopTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint(constants.ContextKeyUserID)

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusBadRequest, common.ErrCodeBadRequest, "Invalid tunnel ID")
		return
	}

	// Fetch tunnel and check ownership
	var tunnel models.Tunnel
	if err := h.db.Where("id = ? AND user_id = ?", tunnelIDUint, userID).First(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusNotFound, common.ErrCodeNotFound, "Tunnel not found")
		return
	}

	// TODO: Implement tunnel stop logic
	tunnel.Status = models.StatusStopping
	tunnel.IsEnabled = false

	if err := h.db.Save(&tunnel).Error; err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Failed to stop tunnel")
		return
	}

	// Return success response
	c.JSON(http.StatusOK, common.NewMessageResponse("Tunnel stopped successfully"))
}