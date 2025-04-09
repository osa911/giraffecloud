package utils

import (
	"os"

	"giraffecloud/internal/api/dto/common"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandleAPIError is a utility function for consistent error handling across the API
// It handles common error types and ensures sensitive error details are only exposed in non-production environments
func HandleAPIError(c *gin.Context, err error, defaultStatus int, defaultCode common.ErrorCode, defaultMessage string) {
	// For record not found errors, return 404
	if err == gorm.ErrRecordNotFound {
		c.JSON(404, common.NewErrorResponse(common.ErrCodeNotFound, "Resource not found", nil))
		return
	}

	// In production, don't expose error details
	var errorDetails interface{} = nil
	if os.Getenv("ENV") != "production" {
		errorDetails = err
	}

	// Return the appropriate error response
	c.JSON(defaultStatus, common.NewErrorResponse(defaultCode, defaultMessage, errorDetails))
}