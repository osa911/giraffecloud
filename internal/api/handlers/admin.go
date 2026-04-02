package handlers

import (
	"net/http"
	"strconv"

	"github.com/osa911/giraffecloud/internal/api/mapper"
	"github.com/osa911/giraffecloud/internal/interfaces"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// AdminHandler handles administrative operations
type AdminHandler struct {
	logger         *logging.Logger
	versionService *service.VersionService
	tunnelService  interfaces.TunnelService
}

// NewAdminHandler creates a new admin handler instance
func NewAdminHandler(versionService *service.VersionService, tunnelService interfaces.TunnelService) *AdminHandler {
	return &AdminHandler{
		logger:         logging.GetGlobalLogger(),
		versionService: versionService,
		tunnelService:  tunnelService,
	}
}

// UpdateVersionConfig updates client version configuration
func (h *AdminHandler) UpdateVersionConfig(c *gin.Context) {
	var config service.ClientVersionConfigUpdate

	if err := c.ShouldBindJSON(&config); err != nil {
		h.logger.Error("Invalid version config request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if config.Channel == "" || config.LatestVersion == "" || config.MinimumVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required fields: channel, latest_version, minimum_version",
		})
		return
	}

	// Set defaults if not provided
	if config.Platform == "" {
		config.Platform = "all"
	}
	if config.Arch == "" {
		config.Arch = "all"
	}

	err := h.versionService.UpdateClientVersionConfig(c.Request.Context(), config)
	if err != nil {
		h.logger.Error("Failed to update version config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update version configuration",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Version config updated successfully for channel=%s platform=%s arch=%s version=%s",
		config.Channel, config.Platform, config.Arch, config.LatestVersion)

	utils.HandleSuccess(c, gin.H{
		"message": "Version configuration updated successfully",
		"config":  config,
	})
}

// GetVersionConfigs returns all version configurations
func (h *AdminHandler) GetVersionConfigs(c *gin.Context) {
	configs, err := h.versionService.ListAllVersionConfigs(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get version configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve version configurations",
		})
		return
	}

	utils.HandleSuccess(c, gin.H{
		"configs": configs,
	})
}

// GetVersionConfig returns version configuration for a specific channel/platform/arch
func (h *AdminHandler) GetVersionConfig(c *gin.Context) {
	channel := c.Query("channel")
	platform := c.Query("platform")
	arch := c.Query("arch")

	if channel == "" {
		channel = "stable"
	}
	if platform == "" {
		platform = "all"
	}
	if arch == "" {
		arch = "all"
	}

	versionInfo, err := h.versionService.GetVersionInfo(c.Request.Context(), "", channel, platform, arch)
	if err != nil {
		h.logger.Error("Failed to get version info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve version configuration",
		})
		return
	}

	utils.HandleSuccess(c, versionInfo)
}

// ListUserTunnels returns all tunnels for a specific user (admin readonly)
func (h *AdminHandler) ListUserTunnels(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	tunnels, err := h.tunnelService.ListTunnels(c.Request.Context(), uint32(userID))
	if err != nil {
		h.logger.Error("Failed to list tunnels for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tunnels"})
		return
	}

	response := mapper.TunnelsToResponses(tunnels)
	utils.HandleSuccess(c, response)
}

// BulkUpdateMinVersion updates minimum version across all version configs
func (h *AdminHandler) BulkUpdateMinVersion(c *gin.Context) {
	var req struct {
		MinimumVersion string `json:"minimum_version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minimum_version is required"})
		return
	}

	configs, err := h.versionService.ListAllVersionConfigs(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list version configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list version configs"})
		return
	}

	updated := 0
	for _, cfg := range configs {
		err := h.versionService.UpdateClientVersionConfig(c.Request.Context(), service.ClientVersionConfigUpdate{
			Channel:           cfg.Channel,
			Platform:          cfg.Platform,
			Arch:              cfg.Arch,
			LatestVersion:     cfg.LatestVersion,
			MinimumVersion:    req.MinimumVersion,
			DownloadURL:       cfg.DownloadURL,
			ReleaseNotes:      cfg.ReleaseNotes,
			AutoUpdateEnabled: cfg.AutoUpdateEnabled,
			ForceUpdate:       cfg.ForceUpdate,
			Metadata:          cfg.Metadata,
		})
		if err != nil {
			h.logger.Error("Failed to update config %s: %v", cfg.ID, err)
			continue
		}
		updated++
	}

	h.logger.Info("Bulk updated minimum version to %s across %d configs", req.MinimumVersion, updated)
	utils.HandleSuccess(c, gin.H{
		"message":         "Minimum version updated",
		"minimum_version": req.MinimumVersion,
		"updated_count":   updated,
	})
}
