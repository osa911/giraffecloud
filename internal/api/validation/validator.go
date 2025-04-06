package validation

import (
	"net/url"
	"regexp"

	"giraffecloud/internal/api/constants"

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
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// FormatValidationError formats validation errors into a user-friendly response
func FormatValidationError(err error) []ValidationError {
	var errors []ValidationError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errors = append(errors, ValidationError{
				Field: e.Field(),
				Tag:   e.Tag(),
				Value: e.Param(),
			})
		}
	}
	return errors
}

// ValidateRequest validates the request body against a struct
func ValidateRequest(v *validator.Validate) gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		var validationErrors []ValidationError

		// Get the validation struct from context
		if val, exists := c.Get(constants.ContextKeyValidate); exists {
			if err = v.Struct(val); err != nil {
				validationErrors = FormatValidationError(err)
				c.JSON(400, gin.H{
					"error": "Validation failed",
					"errors": validationErrors,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}