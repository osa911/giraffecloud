package server

import (
	"giraffecloud/internal/db"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/tunnel"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router       *gin.Engine
	db          *db.Database
	tunnelRouter *tunnel.HybridTunnelRouter // Changed from TunnelServer to HybridTunnelRouter
}

// Repositories holds all repository instances
type Repositories struct {
	User    repository.UserRepository
	Auth    repository.AuthRepository
	Session repository.SessionRepository
	Token   repository.TokenRepository
	Tunnel  repository.TunnelRepository
}

// Config holds the server configuration
type Config struct {
	// Port is the server port number (e.g. "8080")
	Port string
}