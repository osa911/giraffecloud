package utils

import (
	"net/http"

	"giraffecloud/internal/api/dto/common"

	"github.com/gin-gonic/gin"
)

// HandleSuccess sends a success response with data
func HandleSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, common.NewSuccessResponse(data))
}

// HandleCreated sends a created response with data
func HandleCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, common.NewSuccessResponse(data))
}

// HandleMessage sends a success response with just a message
func HandleMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, common.NewMessageResponse(message))
}

// HandleNoContent sends a success response with no content
func HandleNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}