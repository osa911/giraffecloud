package models

import (
	"gorm.io/gorm"
)

type Team struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;not null"`
	Description string
	Users       []User    `gorm:"many2many:team_users;"`
	Tunnels     []Tunnel  `gorm:"foreignKey:TeamID"`
}

// TeamUser represents the many-to-many relationship between teams and users
type TeamUser struct {
	TeamID   uint
	UserID   uint
	Role     TeamRole `gorm:"type:varchar(20);default:'member'"`
	Team     Team     `gorm:"foreignKey:TeamID"`
	User     User     `gorm:"foreignKey:UserID"`
}

type TeamRole string

const (
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
	TeamRoleViewer TeamRole = "viewer"
)