package server

import (
	"giraffecloud/internal/config"
	"giraffecloud/internal/db"
	"giraffecloud/internal/repository"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	cfg    *config.Config
	db     *db.Database
}

// Repositories holds all repository instances
type Repositories struct {
	User    repository.UserRepository
	Auth    repository.AuthRepository
	Session repository.SessionRepository
}