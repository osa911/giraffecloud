package mapper

import (
	"giraffecloud/internal/api/dto/v1/auth"
	"giraffecloud/internal/db/ent"
)

// SessionToSessionResponse maps a Session entity to a SessionResponse DTO
func SessionToSessionResponse(s *ent.Session) auth.SessionResponse {
	return auth.SessionResponse{
		Token:     s.Token,
		ExpiresAt: s.ExpiresAt,
	}
}

// SessionsToSessionResponses maps a slice of Session entities to a slice of SessionResponse DTOs
func SessionsToSessionResponses(sessions []*ent.Session) []auth.SessionResponse {
	result := make([]auth.SessionResponse, len(sessions))
	for i, s := range sessions {
		result[i] = SessionToSessionResponse(s)
	}
	return result
}
