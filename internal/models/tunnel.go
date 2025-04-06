package models

import (
	"time"

	"gorm.io/gorm"
)

type Tunnel struct {
	gorm.Model
	Name        string    `gorm:"uniqueIndex;not null"`
	UserID      uint      `gorm:"not null"`
	User        User      `gorm:"foreignKey:UserID"`
	Protocol    Protocol  `gorm:"type:varchar(10);not null"`
	LocalPort   int       `gorm:"not null"`
	RemoteHost  string    `gorm:"not null"`
	Status      Status    `gorm:"type:varchar(20);default:'inactive'"`
	LastActive  time.Time
	LastError   string
	IsEnabled   bool `gorm:"default:true"`
}

type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
	ProtocolTCP   Protocol = "tcp"
	ProtocolUDP   Protocol = "udp"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusInactive  Status = "inactive"
	StatusError     Status = "error"
	StatusStarting  Status = "starting"
	StatusStopping  Status = "stopping"
	StatusRestarting Status = "restarting"
)

// BeforeCreate is a GORM hook that runs before creating a new tunnel
func (t *Tunnel) BeforeCreate(tx *gorm.DB) error {
	if t.Status == "" {
		t.Status = StatusInactive
	}
	return nil
}