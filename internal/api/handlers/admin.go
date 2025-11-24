package handlers

import (
	"net/http"

	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminHandler handles administrative operations
type AdminHandler struct {
	logger         *logging.Logger
	versionService *service.VersionService
}

// NewAdminHandler creates a new admin handler instance
func NewAdminHandler(versionService *service.VersionService) *AdminHandler {
	return &AdminHandler{
		logger:         logging.GetGlobalLogger(),
		versionService: versionService,
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Version configuration updated successfully",
		"config":  config,
	})
}

// GetVersionConfigs returns all version configurations
func (h *AdminHandler) GetVersionConfigs(c *gin.Context) {
	// This would require adding a method to version service to list all configs
	// For now, return active channels
	channels, err := h.versionService.GetActiveChannels(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get active channels: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve version configurations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channels,
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

	c.JSON(http.StatusOK, versionInfo)
}
