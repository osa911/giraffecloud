package server

import (
	"context"
	"giraffecloud/internal/db"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/service"
	"giraffecloud/internal/tunnel"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router       *gin.Engine
	db           *db.Database
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

// quotaAdapter bridges service.QuotaService to tunnel.QuotaChecker
type quotaAdapter struct{ q service.QuotaService }

func (a quotaAdapter) CheckUser(ctx context.Context, userID uint32) (tunnel.QuotaResult, error) {
	res, err := a.q.CheckUser(ctx, userID)
	if err != nil {
		return tunnel.QuotaResult{}, err
	}
	// Map decision
	var d tunnel.QuotaDecision
	switch res.Decision {
	case service.QuotaAllow:
		d = tunnel.QuotaAllow
	case service.QuotaWarn:
		d = tunnel.QuotaWarn
	case service.QuotaBlock:
		d = tunnel.QuotaBlock
	default:
		d = tunnel.QuotaAllow
	}
	return tunnel.QuotaResult{Decision: d, UsedBytes: res.UsedBytes, LimitBytes: res.LimitBytes}, nil
}

// Config holds the server configuration
type Config struct {
	// Port is the server port number (e.g. "8080")
	Port string
}
