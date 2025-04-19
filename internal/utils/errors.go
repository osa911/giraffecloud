package utils

import (
	commonDto "giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Map error codes to HTTP status codes
var errorStatusMap = map[commonDto.ErrorCode]int{
	commonDto.ErrCodeValidation:      http.StatusBadRequest,
	commonDto.ErrCodeNotFound:        http.StatusNotFound,
	commonDto.ErrCodeUnauthorized:    http.StatusUnauthorized,
	commonDto.ErrCodeForbidden:       http.StatusForbidden,
	commonDto.ErrCodeInternalServer:  http.StatusInternalServerError,
	commonDto.ErrCodeBadRequest:      http.StatusBadRequest,
	commonDto.ErrCodeTooManyRequests: http.StatusTooManyRequests,
	commonDto.ErrCodeConflict:        http.StatusConflict,
}

// LogError logs an error with optional message
func LogError(err error, msg string) {
	logger := logging.GetLogger()
	if msg != "" {
		logger.Error("%s: %v", msg, err)
	} else {
		logger.Error("%v", err)
	}
}

// HandleAPIError handles API errors in a standardized way
func HandleAPIError(c *gin.Context, err error, code commonDto.ErrorCode, msg string) {
	logger := logging.GetLogger()

	// Get status code from map, default to 500 if not found
	status := errorStatusMap[code]
	if status == 0 {
		status = http.StatusInternalServerError
	}

	// Log HTTP error with request details
	logger.LogHTTPError(
		c.Request.Method,
		c.Request.URL.Path,
		GetRealIP(c),
		status,
		msg,
		err,
	)

	// In production, don't expose error details
	var errorDetails interface{} = nil
	if gin.Mode() != gin.ReleaseMode && err != nil {
		errorDetails = err.Error()
	}

	// Return error response
	c.JSON(status, commonDto.NewErrorResponse(code, msg, errorDetails))
}