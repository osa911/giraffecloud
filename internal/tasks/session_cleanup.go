package tasks

import (
	"log"
	"time"

	"giraffecloud/internal/db"
	"giraffecloud/internal/models"

	"gorm.io/gorm"
)

// SessionCleanup handles periodic cleaning of expired sessions
type SessionCleanup struct {
	db *gorm.DB
}

// NewSessionCleanup creates a new session cleanup task
func NewSessionCleanup(database *db.Database) *SessionCleanup {
	return &SessionCleanup{
		db: database.DB,
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

	// Delete expired sessions
	expiredResult := sc.db.Where("expires_at < ?", time.Now()).Delete(&models.Session{})
	log.Printf("Deleted %d expired sessions", expiredResult.RowsAffected)

	// Delete inactive sessions that haven't been used in 30 days
	inactiveResult := sc.db.Where("is_active = ? AND last_used < ?", false, time.Now().AddDate(0, 0, -30)).Delete(&models.Session{})
	log.Printf("Deleted %d inactive sessions", inactiveResult.RowsAffected)

	log.Println("Session cleanup task completed")
}