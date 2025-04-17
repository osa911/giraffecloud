package utils

import (
	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"

	"github.com/gin-gonic/gin"
)

// LogError logs an error with a message using the singleton logger
func LogError(err error, message string) {
	logger := logging.GetLogger()
	logger.Error("%s: %v", message, err)
}

// HandleAPIError is a utility function for consistent error handling across the API
// It handles common error types and ensures sensitive error details are only exposed in non-production environments
func HandleAPIError(c *gin.Context, err error, defaultStatus int, defaultCode common.ErrorCode, defaultMessage string) {
	// For record not found errors, return 404
	if ent.IsNotFound(err) {
		c.JSON(404, common.NewErrorResponse(common.ErrCodeNotFound, "Resource not found", nil))
		return
	}

	// Log the error
	logger := logging.GetLogger()
	logger.LogHTTPError(
		c.Request.Method,
		c.Request.URL.Path,
		GetRealIP(c),
		defaultStatus,
		defaultMessage,
		err,
	)

	// In production, don't expose error details
	var errorDetails interface{} = nil
	if gin.Mode() != gin.ReleaseMode {
		errorDetails = err
	}

	// Return the appropriate error response
	c.JSON(defaultStatus, common.NewErrorResponse(defaultCode, defaultMessage, errorDetails))
}