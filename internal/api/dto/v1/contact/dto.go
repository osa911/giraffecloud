package contact

// ContactRequest represents a contact form submission
type ContactRequest struct {
	Name           string `json:"name" binding:"required,min=2,max=100"`
	Email          string `json:"email" binding:"required,email,max=255"`
	Message        string `json:"message" binding:"required,min=10,max=1000"`
	RecaptchaToken string `json:"recaptcha_token" binding:"required"`
}

// ContactResponse represents the response after submitting a contact form
type ContactResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}
