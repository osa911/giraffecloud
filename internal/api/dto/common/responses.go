package common

// APIResponse is the standard wrapper for all API responses
type APIResponse struct {
	Success bool           `json:"success"`
	Data    interface{}    `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse is a standardized error response structure
type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ValidationError represents a validation error detail
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// MessageResponse is a standardized message response structure
type MessageResponse struct {
	Message string `json:"message"`
}

// Define type for error codes to enforce consistency
type ErrorCode string

// Standard error codes
const (
	ErrCodeValidation      ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden       ErrorCode = "FORBIDDEN"
	ErrCodeInternalServer  ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrCodeBadRequest      ErrorCode = "BAD_REQUEST"
	ErrCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"
	ErrCodeConflict        ErrorCode = "CONFLICT"
)

// NewSuccessResponse creates a new successful API response
func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
	}
}

// NewMessageResponse creates a new success response with a simple message
func NewMessageResponse(message string) APIResponse {
	return NewSuccessResponse(MessageResponse{
		Message: message,
	})
}

// NewErrorResponse creates a new error API response
func NewErrorResponse(code ErrorCode, message string, details interface{}) APIResponse {
	return APIResponse{
		Success: false,
		Error: &ErrorResponse{
			Code:    string(code),
			Message: message,
			Details: details,
		},
	}
}
