package handlers

import (
	"giraffecloud/internal/api/constants"
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/interfaces"
	"giraffecloud/internal/service"
	"giraffecloud/internal/utils"
	"strconv"

	"giraffecloud/internal/logging"

	"github.com/gin-gonic/gin"
)

// TunnelHandler handles tunnel-related HTTP requests
type TunnelHandler struct {
	tunnelService  interfaces.TunnelService
	versionService *service.VersionService
}

// NewTunnelHandler creates a new tunnel handler instance
func NewTunnelHandler(tunnelService interfaces.TunnelService, versionService *service.VersionService) *TunnelHandler {
	return &TunnelHandler{
		tunnelService:  tunnelService,
		versionService: versionService,
	}
}

// GetVersion returns the server version information for client version checking
// This endpoint doesn't require authentication as it's used during tunnel connection
func (h *TunnelHandler) GetVersion(c *gin.Context) {
	logger := logging.GetGlobalLogger()

	// Log request details
	logger.Debug("📥 Version check request:")
	logger.Debug("   Method: %s", c.Request.Method)
	logger.Debug("   URL: %s", c.Request.URL.String())
	logger.Debug("   Headers:")
	for k, v := range c.Request.Header {
		logger.Debug("     %s: %s", k, v)
	}

	// Get client information from query parameters or headers
	clientVersion := c.Query("client_version")
	if clientVersion == "" {
		clientVersion = c.GetHeader("X-Client-Version")
	}

	// Get channel (test mode support)
	channel := c.Query("channel")
	if channel == "" {
		channel = c.GetHeader("X-Release-Channel")
	}

	// Get platform and architecture
	platform := c.Query("platform")
	if platform == "" {
		platform = c.GetHeader("X-Client-Platform")
	}
	arch := c.Query("arch")
	if arch == "" {
		arch = c.GetHeader("X-Client-Arch")
	}

	// Log parsed parameters
	logger.Debug("🔍 Parsed parameters:")
	logger.Debug("   Client Version: %s", clientVersion)
	logger.Debug("   Channel: %s", channel)
	logger.Debug("   Platform: %s", platform)
	logger.Debug("   Architecture: %s", arch)

	// Get version information from service
	versionInfo, err := h.versionService.GetVersionInfo(c.Request.Context(), clientVersion, channel, platform, arch)
	if err != nil {
		logger.Error("❌ Failed to get version info: %v", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve version information",
		})
		return
	}

	// Log response
	logger.Debug("📤 Sending version response:")
	logger.Debug("   Latest Version: %s", versionInfo.LatestVersion)
	logger.Debug("   Minimum Version: %s", versionInfo.MinimumVersion)
	logger.Debug("   Channel: %s", versionInfo.Channel)
	logger.Debug("   Update Available: %v", versionInfo.UpdateAvailable)
	logger.Debug("   Update Required: %v", versionInfo.UpdateRequired)
	logger.Debug("   Download URL: %s", versionInfo.DownloadURL)

	c.JSON(200, versionInfo)
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