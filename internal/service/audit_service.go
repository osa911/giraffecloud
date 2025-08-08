package service

import (
	"context"
	"time"

	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// Authentication events
	AuditEventLogin          AuditEventType = "LOGIN"
	AuditEventLoginFailed    AuditEventType = "LOGIN_FAILED"
	AuditEventLogout         AuditEventType = "LOGOUT"
	AuditEventSessionCreated AuditEventType = "SESSION_CREATED"
	AuditEventSessionRevoked AuditEventType = "SESSION_REVOKED"
	AuditEventSessionExpired AuditEventType = "SESSION_EXPIRED"
	AuditEventTokenRefreshed AuditEventType = "TOKEN_REFRESHED"
	AuditEventTokenInvalid   AuditEventType = "TOKEN_INVALID"
)

// AuditService handles audit logging
type AuditService struct{}

// NewAuditService creates a new audit service
func NewAuditService() *AuditService {
	return &AuditService{}
}

// LogAuthEvent logs an authentication-related event
func (s *AuditService) LogAuthEvent(ctx context.Context, eventType AuditEventType, user *ent.User, ip string, details map[string]interface{}) {
	logger := logging.GetGlobalLogger()

	// Prepare event details
	event := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"event_type": eventType,
		"user_id":    user.ID,
		"user_email": user.Email,
		"ip_address": ip,
	}

	// Add any additional details
	for k, v := range details {
		event[k] = v
	}

	// Log the event
	logger.Info(
		"[AUDIT] %s | User: %d (%s) | IP: %s | Details: %v",
		eventType,
		user.ID,
		user.Email,
		ip,
		details,
	)
}

// LogFailedAuthAttempt logs a failed authentication attempt
func (s *AuditService) LogFailedAuthAttempt(ctx context.Context, c *gin.Context, reason string, err error, extraDetails ...map[string]interface{}) {
	logger := logging.GetGlobalLogger()
	ip := utils.GetRealIP(c)

	details := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"event_type": AuditEventLoginFailed,
		"ip_address": ip,
		"reason":     reason,
	}

	// Add error if present
	if err != nil {
		details["error"] = err.Error()
	}

	// Add any extra details
	if len(extraDetails) > 0 {
		for k, v := range extraDetails[0] {
			details[k] = v
		}
	}

	// Log the event
	logger.Warn(
		"[AUDIT] %s | IP: %s | Reason: %s | Details: %v",
		AuditEventLoginFailed,
		ip,
		reason,
		details,
	)
}

// LogSessionEvent logs a session-related event
func (s *AuditService) LogSessionEvent(ctx context.Context, eventType AuditEventType, session *ent.Session, details map[string]interface{}) {
	logger := logging.GetGlobalLogger()

	// Prepare event details
	event := map[string]interface{}{
		"timestamp":   time.Now().UTC(),
		"event_type":  eventType,
		"session_id":  session.ID,
		"user_id":     session.Edges.Owner.ID,
		"user_email":  session.Edges.Owner.Email,
		"device_name": session.UserAgent,
		"ip_address":  session.IPAddress,
	}

	// Add any additional details
	for k, v := range details {
		event[k] = v
	}

	// Log the event
	logger.Info(
		"[AUDIT] %s | Session: %d | User: %d (%s) | Device: %s | IP: %s | Details: %v",
		eventType,
		session.ID,
		session.Edges.Owner.ID,
		session.Edges.Owner.Email,
		session.UserAgent,
		session.IPAddress,
		details,
	)
}
