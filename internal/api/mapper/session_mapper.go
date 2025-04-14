package mapper

import (
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/db/ent"
)

// SessionToSessionResponse converts an Ent Session entity to a SessionResponse DTO
func SessionToSessionResponse(s *ent.Session) *auth.SessionResponse {
	if s == nil {
		return nil
	}

	return &auth.SessionResponse{
		Token:     s.Token,
		ExpiresAt: s.ExpiresAt,
	}
}

// SessionsToSessionResponses converts a slice of Ent Session entities to SessionResponse DTOs
func SessionsToSessionResponses(sessions []*ent.Session) []auth.SessionResponse {
	result := make([]auth.SessionResponse, len(sessions))
	for i, s := range sessions {
		result[i] = *SessionToSessionResponse(s)
	}
	return result
}