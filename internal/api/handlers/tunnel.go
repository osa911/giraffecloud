package handlers

import (
	"net/http"
	"strconv"

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

type CreateTunnelRequest struct {
	Name        string `json:"name" binding:"required"`
	Protocol    string `json:"protocol" binding:"required,oneof=http https tcp udp"`
	LocalPort   int    `json:"localPort" binding:"required,min=1,max=65535"`
	RemoteHost  string `json:"remoteHost" binding:"required"`
	TeamID      uint   `json:"teamId" binding:"required"`
}

type UpdateTunnelRequest struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	IsEnabled  bool   `json:"isEnabled"`
}

func (h *TunnelHandler) ListTunnels(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get user's teams
	var teamIDs []uint
	if err := h.db.Model(&models.TeamUser{}).Where("user_id = ?", userID).Pluck("team_id", &teamIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user's teams"})
		return
	}

	var tunnels []models.Tunnel
	if err := h.db.Where("team_id IN ?", teamIDs).Find(&tunnels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tunnels"})
		return
	}

	c.JSON(http.StatusOK, tunnels)
}

func (h *TunnelHandler) CreateTunnel(c *gin.Context) {
	userID := c.GetUint("userID")

	var req CreateTunnelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", req.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	// Check if user has permission to create tunnels
	if teamUser.Role == models.TeamRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot create tunnels"})
		return
	}

	tunnel := models.Tunnel{
		Name:       req.Name,
		UserID:     userID,
		Protocol:   models.Protocol(req.Protocol),
		LocalPort:  req.LocalPort,
		RemoteHost: req.RemoteHost,
		Status:     models.StatusInactive,
		IsEnabled:  true,
		TeamID:     req.TeamID,
	}

	if err := h.db.Create(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tunnel"})
		return
	}

	c.JSON(http.StatusCreated, tunnel)
}

func (h *TunnelHandler) GetTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tunnel ID"})
		return
	}

	// Get tunnel with team information
	var tunnel models.Tunnel
	if err := h.db.Preload("Team").First(&tunnel, tunnelIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tunnel not found"})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", tunnel.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	c.JSON(http.StatusOK, tunnel)
}

func (h *TunnelHandler) UpdateTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tunnel ID"})
		return
	}

	var req UpdateTunnelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tunnel not found"})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", tunnel.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	// Check if user has permission to update tunnels
	if teamUser.Role == models.TeamRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot update tunnels"})
		return
	}

	// Update fields
	if req.Name != "" {
		tunnel.Name = req.Name
	}
	if req.Protocol != "" {
		tunnel.Protocol = models.Protocol(req.Protocol)
	}
	if req.LocalPort > 0 {
		tunnel.LocalPort = req.LocalPort
	}
	if req.RemoteHost != "" {
		tunnel.RemoteHost = req.RemoteHost
	}
	tunnel.IsEnabled = req.IsEnabled

	if err := h.db.Save(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tunnel"})
		return
	}

	c.JSON(http.StatusOK, tunnel)
}

func (h *TunnelHandler) DeleteTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tunnel ID"})
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tunnel not found"})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", tunnel.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	// Check if user has permission to delete tunnels
	if teamUser.Role == models.TeamRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot delete tunnels"})
		return
	}

	if err := h.db.Delete(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tunnel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tunnel deleted successfully"})
}

func (h *TunnelHandler) StartTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tunnel ID"})
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tunnel not found"})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", tunnel.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	// Check if user has permission to manage tunnels
	if teamUser.Role == models.TeamRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot manage tunnels"})
		return
	}

	// TODO: Implement tunnel start logic
	tunnel.Status = models.StatusStarting
	tunnel.IsEnabled = true

	if err := h.db.Save(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start tunnel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tunnel started successfully"})
}

func (h *TunnelHandler) StopTunnel(c *gin.Context) {
	tunnelID := c.Param("id")
	userID := c.GetUint("userID")

	// Convert tunnelID to uint
	tunnelIDUint, err := strconv.ParseUint(tunnelID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tunnel ID"})
		return
	}

	var tunnel models.Tunnel
	if err := h.db.First(&tunnel, tunnelIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tunnel not found"})
		return
	}

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", tunnel.TeamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	// Check if user has permission to manage tunnels
	if teamUser.Role == models.TeamRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot manage tunnels"})
		return
	}

	// TODO: Implement tunnel stop logic
	tunnel.Status = models.StatusStopping
	tunnel.IsEnabled = false

	if err := h.db.Save(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop tunnel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tunnel stopped successfully"})
}