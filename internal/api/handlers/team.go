package handlers

import (
	"net/http"
	"strconv"

	"giraffecloud/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TeamHandler struct {
	db *gorm.DB
}

func NewTeamHandler(db *gorm.DB) *TeamHandler {
	return &TeamHandler{db: db}
}

type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UpdateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AddTeamMemberRequest struct {
	UserID uint   `json:"userId" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=admin member viewer"`
}

func (h *TeamHandler) ListTeams(c *gin.Context) {
	userID := c.GetUint("userID")

	var teams []models.Team
	if err := h.db.Joins("JOIN team_users ON teams.id = team_users.team_id").
		Where("team_users.user_id = ?", userID).
		Find(&teams).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch teams"})
		return
	}

	c.JSON(http.StatusOK, teams)
}

func (h *TeamHandler) CreateTeam(c *gin.Context) {
	userID := c.GetUint("userID")

	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create team
	team := models.Team{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.db.Create(&team).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	// Add creator as admin
	teamUser := models.TeamUser{
		TeamID: team.ID,
		UserID: userID,
		Role:   models.TeamRoleAdmin,
	}

	if err := h.db.Create(&teamUser).Error; err != nil {
		h.db.Delete(&team) // Rollback team creation
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add team member"})
		return
	}

	c.JSON(http.StatusCreated, team)
}

func (h *TeamHandler) GetTeam(c *gin.Context) {
	teamID := c.Param("id")
	userID := c.GetUint("userID")

	// Check if user is a member of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", teamID, userID).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this team"})
		return
	}

	var team models.Team
	if err := h.db.First(&team, teamID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	// Get team members
	var members []models.TeamUser
	if err := h.db.Preload("User").Where("team_id = ?", teamID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch team members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"team":    team,
		"members": members,
	})
}

func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	teamID := c.Param("id")
	userID := c.GetUint("userID")

	// Check if user is an admin of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, models.TeamRoleAdmin).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to update team"})
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var team models.Team
	if err := h.db.First(&team, teamID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	// Update fields
	if req.Name != "" {
		team.Name = req.Name
	}
	if req.Description != "" {
		team.Description = req.Description
	}

	if err := h.db.Save(&team).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	c.JSON(http.StatusOK, team)
}

func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	teamID := c.Param("id")
	userID := c.GetUint("userID")

	// Check if user is an admin of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, models.TeamRoleAdmin).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to delete team"})
		return
	}

	if err := h.db.Delete(&models.Team{}, teamID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted successfully"})
}

func (h *TeamHandler) AddTeamMember(c *gin.Context) {
	teamID := c.Param("id")
	userID := c.GetUint("userID")

	// Check if user is an admin of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, models.TeamRoleAdmin).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to add team members"})
		return
	}

	var req AddTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is already a member
	var existingMember models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ?", teamID, req.UserID).First(&existingMember).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this team"})
		return
	}

	// Add new member
	teamIDUint, err := strconv.ParseUint(teamID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	newMember := models.TeamUser{
		TeamID: uint(teamIDUint),
		UserID: req.UserID,
		Role:   models.TeamRole(req.Role),
	}

	if err := h.db.Create(&newMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add team member"})
		return
	}

	c.JSON(http.StatusCreated, newMember)
}

func (h *TeamHandler) RemoveTeamMember(c *gin.Context) {
	teamID := c.Param("id")
	memberID := c.Param("userId")
	userID := c.GetUint("userID")

	// Check if user is an admin of the team
	var teamUser models.TeamUser
	if err := h.db.Where("team_id = ? AND user_id = ? AND role = ?", teamID, userID, models.TeamRoleAdmin).First(&teamUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to remove team members"})
		return
	}

	// Check if trying to remove the last admin
	var adminCount int64
	if err := h.db.Model(&models.TeamUser{}).Where("team_id = ? AND role = ?", teamID, models.TeamRoleAdmin).Count(&adminCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check admin count"})
		return
	}

	if adminCount == 1 {
		var memberToRemove models.TeamUser
		if err := h.db.Where("team_id = ? AND user_id = ?", teamID, memberID).First(&memberToRemove).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team member not found"})
			return
		}

		if memberToRemove.Role == models.TeamRoleAdmin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove the last admin"})
			return
		}
	}

	if err := h.db.Where("team_id = ? AND user_id = ?", teamID, memberID).Delete(&models.TeamUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove team member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team member removed successfully"})
}