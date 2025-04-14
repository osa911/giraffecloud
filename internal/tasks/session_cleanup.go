package tasks

import (
	"context"
	"log"
	"time"

	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/session"
)

// SessionCleanup handles periodic cleaning of expired sessions
type SessionCleanup struct {
	client *ent.Client
}

// NewSessionCleanup creates a new session cleanup task
func NewSessionCleanup(client *ent.Client) *SessionCleanup {
	return &SessionCleanup{
		client: client,
	}
}

// Start begins the session cleanup task in the background
func (sc *SessionCleanup) Start() {
	go sc.runPeriodically()
}

// runPeriodically runs the cleanup task at regular intervals
func (sc *SessionCleanup) runPeriodically() {
	// Run immediately on startup
	sc.cleanup()

	// Then run every 12 hours
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sc.cleanup()
	}
}

// cleanup performs the actual session cleanup
func (sc *SessionCleanup) cleanup() {
	log.Println("Starting session cleanup task")

	ctx := context.Background()

	// Delete expired sessions
	expiredCount, err := sc.client.Session.Delete().
		Where(session.ExpiresAtLT(time.Now())).
		Exec(ctx)
	if err != nil {
		log.Printf("Error deleting expired sessions: %v", err)
	} else {
		log.Printf("Deleted %d expired sessions", expiredCount)
	}

	// Delete inactive sessions that haven't been used in 30 days
	inactiveCount, err := sc.client.Session.Delete().
		Where(
			session.IsActive(false),
			session.LastUsedLT(time.Now().AddDate(0, 0, -30)),
		).
		Exec(ctx)
	if err != nil {
		log.Printf("Error deleting inactive sessions: %v", err)
	} else {
		log.Printf("Deleted %d inactive sessions", inactiveCount)
	}

	log.Println("Session cleanup task completed")
}