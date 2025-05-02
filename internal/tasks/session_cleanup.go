package tasks

import (
	"context"
	"log"
	"sync"
	"time"

	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/session"
)

// SessionCleanup handles periodic cleaning of expired sessions
type SessionCleanup struct {
	client *ent.Client
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewSessionCleanup creates a new session cleanup task
func NewSessionCleanup(client *ent.Client) *SessionCleanup {
	return &SessionCleanup{
		client: client,
		done:   make(chan struct{}),
	}
}

// Start begins the session cleanup task in the background
func (sc *SessionCleanup) Start() {
	sc.wg.Add(1)
	go sc.runPeriodically()
}

// Stop gracefully stops the session cleanup task
func (sc *SessionCleanup) Stop() {
	close(sc.done)
	sc.wg.Wait()
}

// runPeriodically runs the cleanup task at regular intervals
func (sc *SessionCleanup) runPeriodically() {
	defer sc.wg.Done()

	// Run immediately on startup
	if err := sc.cleanup(); err != nil {
		log.Printf("Initial session cleanup failed: %v", err)
	}

	// Then run every 12 hours
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sc.cleanup(); err != nil {
				log.Printf("Periodic session cleanup failed: %v", err)
			}
		case <-sc.done:
			return
		}
	}
}

// cleanup performs the actual session cleanup
func (sc *SessionCleanup) cleanup() error {
	log.Println("Starting session cleanup task")

	ctx := context.Background()

	// Delete expired sessions
	expiredCount, err := sc.client.Session.Delete().
		Where(session.ExpiresAtLT(time.Now())).
		Exec(ctx)
	if err != nil {
		log.Printf("Error deleting expired sessions: %v", err)
		return err
	}
	log.Printf("Deleted %d expired sessions", expiredCount)

	// Delete inactive sessions that haven't been used in 30 days
	inactiveCount, err := sc.client.Session.Delete().
		Where(
			session.IsActive(false),
			session.LastUsedLT(time.Now().AddDate(0, 0, -30)),
		).
		Exec(ctx)
	if err != nil {
		log.Printf("Error deleting inactive sessions: %v", err)
		return err
	}
	log.Printf("Deleted %d inactive sessions", inactiveCount)

	log.Println("Session cleanup task completed")
	return nil
}