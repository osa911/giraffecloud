package server

import (
	"fmt"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/server/routes"
	"giraffecloud/internal/service"
	"io"
	"os"

	"github.com/gin-gonic/gin"
)

// NewServer creates a new server instance
func NewServer(db *db.Database) (*Server, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	engine := gin.New()

	// Configure trusted proxies
	engine.SetTrustedProxies([]string{
		"127.0.0.1",      // localhost
		"::1",            // localhost IPv6
		"172.20.0.0/16",  // Docker network
		"192.168.0.0/16", // private network
		"10.0.0.0/8",     // private network
	})

	return &Server{
		router: engine,
		db:     db,
	}, nil
}

// Init initializes the server
func (s *Server) Init() error {
	// Disable default logger
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	// Set up global middleware
	logger := logging.GetLogger()
	routes.SetupGlobalMiddleware(s.router, logger)

	// Initialize services
	auditService := service.NewAuditService()
	csrfService := service.NewCSRFService()

	// Initialize Caddy service
	caddyService := service.NewCaddyService(&service.CaddyConfig{
		AdminAPI: os.Getenv("CADDY_ADMIN_API"),
	})

	// Load initial Caddy configuration
	if err := caddyService.LoadConfig(); err != nil {
		return err
	}

	// Initialize repositories
	repos := s.initializeRepositories()

	// Initialize token service
	tokenService := service.NewTokenService(repos.Token)

	// Initialize tunnel service with Caddy service
	tunnelService := service.NewTunnelService(repos.Tunnel, caddyService)

	// Initialize handlers
	handlers := &routes.Handlers{
		Auth:    handlers.NewAuthHandler(repos.Auth, repos.Session, csrfService, auditService),
		User:    handlers.NewUserHandler(repos.User),
		Health:  handlers.NewHealthHandler(s.db.DB),
		Session: handlers.NewSessionHandler(repos.Session),
		Token:   handlers.NewTokenHandler(tokenService),
		Tunnel:  handlers.NewTunnelHandler(tunnelService),
	}

	// Initialize middleware
	middleware := &routes.Middleware{
		Validation: middleware.NewValidationMiddleware(),
		Auth:       middleware.NewAuthMiddleware(tokenService, repos.Auth, repos.Session, repos.User),
		CSRF:       csrfService,
	}

	// Set up all routes
	routes.Setup(s.router, handlers, middleware)

	return nil
}

// initializeRepositories creates all repository instances
func (s *Server) initializeRepositories() *Repositories {
	return &Repositories{
		User:    repository.NewUserRepository(s.db.DB),
		Auth:    repository.NewAuthRepository(s.db.DB),
		Session: repository.NewSessionRepository(s.db.DB),
		Token:   repository.NewTokenRepository(s.db.DB),
		Tunnel:  repository.NewTunnelRepository(s.db.DB),
	}
}

// Start starts the server
func (s *Server) Start(cfg *Config) error {
	logger := logging.GetLogger()
	logger.Info("Starting server on port " + cfg.Port)
	return s.router.Run(":" + cfg.Port)
}