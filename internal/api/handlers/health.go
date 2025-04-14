package handlers

import (
	"context"
	"net/http"

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
		utils.HandleAPIError(c, err, http.StatusInternalServerError, common.ErrCodeInternalServer, "Database connection error")
		return
	}

	c.JSON(http.StatusOK, common.NewMessageResponse("Health check OK"))
}