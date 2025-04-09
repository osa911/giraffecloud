package handlers

import (
	"net/http"

	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c *gin.Context) {
	// Test DB connection
	sqlDB, err := h.db.DB()
	if err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database configuration error")
		return
	}

	if err := sqlDB.Ping(); err != nil {
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database connection error")
		return
	}

	c.JSON(http.StatusOK, common.NewMessageResponse("Health check OK"))
}