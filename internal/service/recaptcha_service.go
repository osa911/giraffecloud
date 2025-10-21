package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// RecaptchaService handles reCAPTCHA verification
type RecaptchaService struct {
	secretKey string
	client    *http.Client
}

// NewRecaptchaService creates a new reCAPTCHA service
func NewRecaptchaService() *RecaptchaService {
	return &RecaptchaService{
		secretKey: os.Getenv("RECAPTCHA_SECRET_KEY"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// recaptchaResponse represents the response from Google's reCAPTCHA API
type recaptchaResponse struct {
	Success     bool     `json:"success"`
	Score       float64  `json:"score"`
	Action      string   `json:"action"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
}

// VerifyToken verifies a reCAPTCHA token
func (s *RecaptchaService) VerifyToken(token string, minScore float64) (bool, error) {
	if s.secretKey == "" {
		return false, fmt.Errorf("reCAPTCHA secret key not configured")
	}

	if token == "" {
		return false, fmt.Errorf("reCAPTCHA token is required")
	}

	// Prepare the request
	data := url.Values{}
	data.Set("secret", s.secretKey)
	data.Set("response", token)

	// Send verification request
	resp, err := s.client.PostForm("https://www.google.com/recaptcha/api/siteverify", data)
	if err != nil {
		return false, fmt.Errorf("failed to verify reCAPTCHA: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result recaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to parse reCAPTCHA response: %w", err)
	}

	// Check if verification was successful
	if !result.Success {
		return false, fmt.Errorf("reCAPTCHA verification failed: %v", result.ErrorCodes)
	}

	// Check score (for reCAPTCHA v3)
	if result.Score < minScore {
		return false, fmt.Errorf("reCAPTCHA score too low: %.2f < %.2f", result.Score, minScore)
	}

	return true, nil
}
