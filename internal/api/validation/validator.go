package validation

import (
	"net/url"
	"regexp"

	"giraffecloud/internal/api/constants"
	commonDto "giraffecloud/internal/api/dto/common"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	nameRegex     = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]{2,50}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)
	urlRegex      = regexp.MustCompile(`^(https?:\/\/)?([\da-z\.-]+)\.([a-z\.]{2,6})([\/\w \.-]*)*\/?$`)
)

// RegisterValidators registers custom validators
func RegisterValidators(v *validator.Validate) {
	v.RegisterValidation("email", validateEmail)
	v.RegisterValidation("name", validateName)
	v.RegisterValidation("username", validateUsername)
	v.RegisterValidation("url", validateURL)
}

// validateEmail checks if the email is valid
func validateEmail(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	return emailRegex.MatchString(email)
}

// validateName checks if the name is valid
func validateName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	return nameRegex.MatchString(name)
}

// validateUsername checks if the username is valid
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	return usernameRegex.MatchString(username)
}

// validateURL checks if the URL is valid
func validateURL(fl validator.FieldLevel) bool {
	urlStr := fl.Field().String()
	if urlStr == "" {
		return true // Allow empty URLs
	}
	_, err := url.ParseRequestURI(urlStr)
	return err == nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// FormatValidationError formats validation errors into a user-friendly response
func FormatValidationError(err error) []ValidationError {
	var errors []ValidationError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			message := getValidationErrorMessage(e)
			errors = append(errors, ValidationError{
				Field:   e.Field(),
				Tag:     e.Tag(),
				Value:   e.Param(),
				Message: message,
			})
		}
	}
	return errors
}

// getValidationErrorMessage returns a user-friendly error message based on the validation error
func getValidationErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "name":
		return "Name must be 2-50 characters long and can only contain letters, numbers, spaces, hyphens, and underscores"
	case "username":
		return "Username must be 3-30 characters long and can only contain letters, numbers, hyphens, and underscores"
	case "url":
		return "Invalid URL format"
	case "min":
		return "Value is too short"
	case "max":
		return "Value is too long"
	default:
		return "Invalid value"
	}
}

// ValidateRequest validates the request body against a struct
func ValidateRequest(v *validator.Validate) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the validation struct from context
		if val, exists := c.Get(constants.ContextKeyValidate); exists {
			if err := v.Struct(val); err != nil {
				validationErrors := FormatValidationError(err)
				// Pass validation errors as details in the error response
				c.Set("validation_errors", validationErrors)
				utils.HandleAPIError(c, err, commonDto.ErrCodeValidation, "Validation failed")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}