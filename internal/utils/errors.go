package utils

import (
	"fmt"
	"os"
	"time"

	"giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/db/ent"

	"github.com/gin-gonic/gin"
)

// ANSI color codes for terminal output
const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

// LogError logs an error with a message if logging is enabled
func LogError(err error, message string) {
	if os.Getenv("LOG_REQUESTS") == "true" {
		errorLog := fmt.Sprintf(
			"[GFC-API-ERROR] %s | %s: %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			message,
			err.Error(),
		)
		fmt.Print(errorLog)
	}
}

// HandleAPIError is a utility function for consistent error handling across the API
// It handles common error types and ensures sensitive error details are only exposed in non-production environments
func HandleAPIError(c *gin.Context, err error, defaultStatus int, defaultCode common.ErrorCode, defaultMessage string) {
	// For record not found errors, return 404
	if ent.IsNotFound(err) {
		c.JSON(404, common.NewErrorResponse(common.ErrCodeNotFound, "Resource not found", nil))
		return
	}

	// Log the error if configured to do so
	if os.Getenv("LOG_REQUESTS") == "true" {
		// Get contextual info
		path := c.Request.URL.Path
		method := c.Request.Method
		clientIP := GetRealIP(c)

		// Get colored status code based on severity
		statusColor := red // Default to red for errors
		statusFormatted := fmt.Sprintf("%s %3d %s", statusColor, defaultStatus, reset)

		// Get colored method
		methodColor := blue
		switch method {
		case "GET":
			methodColor = blue
		case "POST":
			methodColor = cyan
		case "PUT", "PATCH":
			methodColor = yellow
		case "DELETE":
			methodColor = red
		}
		methodFormatted := fmt.Sprintf("%s %s %s", methodColor, method, reset)

		// Format the error log with colors
		errorLog := fmt.Sprintf(
			"[GFC-API-ERROR] %s | %s | %15s | %-17s %s | %s: %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusFormatted,
			clientIP,
			methodFormatted,
			path,
			defaultMessage,
			err.Error(),
		)

		// Print to stdout directly instead of using gin's writer
		fmt.Print(errorLog)
	}

	// In production, don't expose error details
	var errorDetails interface{} = nil
	if os.Getenv("ENV") != "production" {
		errorDetails = err
	}

	// Return the appropriate error response
	c.JSON(defaultStatus, common.NewErrorResponse(defaultCode, defaultMessage, errorDetails))
}