package server

import (
	"fmt"
	"os"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/caddy"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/server/routes"
	"giraffecloud/internal/service"

	"github.com/gin-gonic/gin"
)

// NewServer creates a new server instance
func NewServer(db *db.Database) (*Server, error) {
	logger := logging.GetGlobalLogger()

	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	// Set Gin mode based on environment
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
		logger.Info("Server initializing in PRODUCTION mode")
	} else {
		gin.SetMode(gin.DebugMode)
		logger.Info("Server initializing in DEVELOPMENT mode")
	}

	logger.Info("Creating Gin engine...")
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Configure trusted proxies
	logger.Info("Configuring trusted proxies...")
	engine.SetTrustedProxies([]string{
		"127.0.0.1",      // localhost
		"::1",            // localhost IPv6
		"172.20.0.0/16",  // Docker network
		"192.168.0.0/16", // private network
		"10.0.0.0/8",     // private network
	})
	logger.Info("Trusted proxies configured")

	return &Server{
		router: engine,
		db:     db,
	}, nil
}

// Init initializes the server
func (s *Server) Init() error {
	// Get the already configured logger
	logger := logging.GetGlobalLogger()
	logger.Info("Global logger initialized")

	// Set up global middleware with our custom logger
	logger.Info("Setting up global middleware...")
	routes.SetupGlobalMiddleware(s.router, logger)
	logger.Info("Global middleware setup completed")

	// Initialize services
	logger.Info("Initializing core services...")
	auditService := service.NewAuditService()
	csrfService := service.NewCSRFService()
	logger.Info("Core services initialized")

	var caddyService service.CaddyService
	if os.Getenv("ENV") == "production" {
		// Initialize Caddy service using standardized configuration
		logger.Info("Initializing Caddy service...")
		logger.Info("Using Caddy config path: %s", caddy.CaddyPaths.Config)

		caddyService = service.NewCaddyService()

		// Load initial Caddy configuration
		logger.Info("Loading initial Caddy configuration...")
		if err := caddyService.LoadConfig(); err != nil {
			logger.Error("Failed to load Caddy configuration: %v", err)
			return fmt.Errorf("failed to load Caddy configuration: %w", err)
		}
		logger.Info("Caddy configuration loaded successfully")
	} else {
		logger.Info("Skipping Caddy initialization in development mode")
	}

	// Initialize repositories
	logger.Info("Initializing repositories...")
	repos := s.initializeRepositories()
	logger.Info("Repositories initialized")

	// Initialize token service
	logger.Info("Initializing token service...")
	tokenService := service.NewTokenService(repos.Token)
	logger.Info("Token service initialized")

	// Initialize tunnel service
	logger.Info("Initializing tunnel service...")
	tunnelService := service.NewTunnelService(repos.Tunnel, caddyService)
	logger.Info("Tunnel service initialized")

	// Initialize handlers
	logger.Info("Initializing handlers...")
	handlers := &routes.Handlers{
		Auth:    handlers.NewAuthHandler(repos.Auth, repos.Session, csrfService, auditService),
		User:    handlers.NewUserHandler(repos.User),
		Health:  handlers.NewHealthHandler(s.db.DB),
		Session: handlers.NewSessionHandler(repos.Session),
		Token:   handlers.NewTokenHandler(tokenService),
		Tunnel:  handlers.NewTunnelHandler(tunnelService),
	}
	logger.Info("Handlers initialized")

	// Initialize middleware
	logger.Info("Initializing middleware components...")
	middleware := &routes.Middleware{
		Validation: middleware.NewValidationMiddleware(),
		Auth:       middleware.NewAuthMiddleware(tokenService, repos.Auth, repos.Session, repos.User),
		CSRF:       csrfService,
	}
	logger.Info("Middleware components initialized")

	// Set up all routes
	logger.Info("Setting up routes...")
	routes.Setup(s.router, handlers, middleware)
	logger.Info("Routes setup completed")

	return nil
}

// initializeRepositories creates all repository instances
func (s *Server) initializeRepositories() *Repositories {
	logger := logging.GetGlobalLogger()

	logger.Info("Creating User repository...")
	userRepo := repository.NewUserRepository(s.db.DB)

	logger.Info("Creating Auth repository...")
	authRepo := repository.NewAuthRepository(s.db.DB)

	logger.Info("Creating Session repository...")
	sessionRepo := repository.NewSessionRepository(s.db.DB)

	logger.Info("Creating Token repository...")
	tokenRepo := repository.NewTokenRepository(s.db.DB)

	logger.Info("Creating Tunnel repository...")
	tunnelRepo := repository.NewTunnelRepository(s.db.DB)

	return &Repositories{
		User:    userRepo,
		Auth:    authRepo,
		Session: sessionRepo,
		Token:   tokenRepo,
		Tunnel:  tunnelRepo,
	}
}

// Start starts the server
func (s *Server) Start(cfg *Config) error {
	logger := logging.GetGlobalLogger()
	logger.Info("Starting server on port " + cfg.Port)

	// Log important environment variables
	logger.Info("Server configuration:")
	logger.Info("- Port: %s", cfg.Port)
	logger.Info("- Environment: %s", os.Getenv("ENV"))
	logger.Info("- Database URL: %s", os.Getenv("DATABASE_URL"))
	if os.Getenv("ENV") == "production" {
		logger.Info("- Caddy Config: %s", caddy.CaddyPaths.Config)
	}

	return s.router.Run(":" + cfg.Port)
}