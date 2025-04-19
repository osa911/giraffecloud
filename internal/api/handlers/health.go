package handlers

import (
	"context"

	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	client *ent.Client
}

func NewHealthHandler(client *ent.Client) *HealthHandler {
	return &HealthHandler{client: client}
}

func (h *HealthHandler) Check(c *gin.Context) {
	// Test DB connection by running a simple query
	_, err := h.client.User.Query().Count(context.Background())
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Database connection error")
		return
	}

	utils.HandleMessage(c, "Health check OK")
}