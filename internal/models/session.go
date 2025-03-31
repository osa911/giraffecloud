package models

import (
	"time"

	"gorm.io/gorm"
)

type Session struct {
	gorm.Model
	UserID      uint      `gorm:"not null"`
	User        User      `gorm:"foreignKey:UserID"`
	Token       string    `gorm:"uniqueIndex;not null"`
	DeviceName  string    `gorm:"not null"`
	DeviceID    string    `gorm:"uniqueIndex;not null"`
	LastUsed    time.Time
	ExpiresAt   time.Time
	IsActive    bool `gorm:"default:true"`
	IPAddress   string
	UserAgent   string
}

// BeforeCreate is a GORM hook that runs before creating a new session
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.LastUsed.IsZero() {
		s.LastUsed = time.Now()
	}
	if s.ExpiresAt.IsZero() {
		// Default to 30 days from creation
		s.ExpiresAt = time.Now().AddDate(0, 0, 30)
	}
	return nil
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Refresh updates the last used time and extends the expiration
func (s *Session) Refresh() {
	s.LastUsed = time.Now()
	s.ExpiresAt = time.Now().AddDate(0, 0, 30)
}