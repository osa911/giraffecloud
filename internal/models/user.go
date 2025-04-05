package models

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User represents a user in the system
type User struct {
	gorm.Model
	FirebaseUID  string    `gorm:"uniqueIndex;not null"`
	Email        string    `gorm:"uniqueIndex;not null"`
	Name         string    `gorm:"not null"`
	Role         Role      `gorm:"default:user;not null"`
	IsActive     bool      `gorm:"default:true;not null"`
	LastLogin    time.Time
	LastActivity time.Time
	LastLoginIP  string    `gorm:"type:varchar(45)"` // IPv6 addresses can be up to 45 characters
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Teams        []Team    `gorm:"many2many:team_users;"`
	TeamUsers    []TeamUser
}

// BeforeCreate is a GORM hook that runs before creating a new user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	return nil
}