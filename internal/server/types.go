package server

import (
	"context"

	"github.com/osa911/giraffecloud/internal/config"
	"github.com/osa911/giraffecloud/internal/db"
	"github.com/osa911/giraffecloud/internal/repository"
	"github.com/osa911/giraffecloud/internal/service"
	"github.com/osa911/giraffecloud/internal/tunnel"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router       *gin.Engine
	db           *db.Database
	tunnelRouter *tunnel.HybridTunnelRouter // Changed from TunnelServer to HybridTunnelRouter
	config       *config.Config
	usageService service.UsageService
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
