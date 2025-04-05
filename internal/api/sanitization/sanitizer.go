package sanitization

import (
	"html/template"
	"regexp"
	"strings"
)

// SanitizeString removes potentially dangerous characters from a string
func SanitizeString(input string) string {
	// Remove HTML tags
	safe := template.HTMLEscapeString(input)

	// Remove multiple spaces
	safe = regexp.MustCompile(`\s+`).ReplaceAllString(safe, " ")

	// Trim whitespace
	safe = strings.TrimSpace(safe)

	return safe
}

// SanitizeEmail removes potentially dangerous characters from an email address
func SanitizeEmail(input string) string {
	// Convert to lowercase
	email := strings.ToLower(input)

	// Remove whitespace
	email = strings.TrimSpace(email)

	// Remove any HTML tags
	email = template.HTMLEscapeString(email)

	return email
}

// SanitizeName removes potentially dangerous characters from a name
func SanitizeName(input string) string {
	// Remove HTML tags
	safe := template.HTMLEscapeString(input)

	// Remove special characters except spaces, hyphens, and underscores
	safe = regexp.MustCompile(`[^a-zA-Z0-9\s\-_]`).ReplaceAllString(safe, "")

	// Remove multiple spaces
	safe = regexp.MustCompile(`\s+`).ReplaceAllString(safe, " ")

	// Trim whitespace
	safe = strings.TrimSpace(safe)

	return safe
}
