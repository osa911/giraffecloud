package service

import (
	"crypto/rand"
	"encoding/base64"
)

// CSRFService handles CSRF token operations
type CSRFService interface {
	GenerateToken() (string, error)
	ValidateToken(token, header string) bool
}

type csrfService struct{}

// NewCSRFService creates a new CSRF service
func NewCSRFService() CSRFService {
	return &csrfService{}
}

// GenerateToken generates a secure random token
func (s *csrfService) GenerateToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// Use RawURLEncoding to avoid URL-unsafe characters without encoding
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ValidateToken validates the CSRF token against the header
func (s *csrfService) ValidateToken(token, header string) bool {
	return token != "" && header != "" && token == header
}
