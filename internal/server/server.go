package server

import (
	"fmt"
	"io"
	"os"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/config"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/server/routes"
	"giraffecloud/internal/service"

	"github.com/gin-gonic/gin"
)

// NewServer creates a new server instance
func NewServer(cfg *config.Config, db *db.Database) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	// Create server instance
	s := &Server{
		router: gin.New(),
		cfg:    cfg,
		db:     db,
	}

	// Initialize all components and set up routes
	if err := s.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	return s, nil
}

// initialize sets up all the server components
func (s *Server) initialize() error {
	// Disable default logger
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard

	// Set up global middleware
	logger := logging.GetLogger()
	routes.SetupGlobalMiddleware(s.router, logger)

	// Initialize services
	auditService := service.NewAuditService()
	csrfService := service.NewCSRFService()

	// Initialize repositories
	repos := s.initializeRepositories()

	// Initialize handlers
	handlers := &routes.Handlers{
		Auth:    handlers.NewAuthHandler(repos.Auth, repos.Session, csrfService, auditService),
		User:    handlers.NewUserHandler(repos.User),
		Health:  handlers.NewHealthHandler(s.db.DB),
		Session: handlers.NewSessionHandler(repos.Session),
	}

	// Initialize middleware
	middleware := &routes.Middleware{
		Validation: middleware.NewValidationMiddleware(),
		Auth:       middleware.NewAuthMiddleware(),
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
	}
}

// Start starts the server
func (s *Server) Start() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger := logging.GetLogger()
	logger.Info("Starting server on port " + port)
	return s.router.Run(":" + port)
}