package server

import (
	"io"
	"os"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/db"
	"giraffecloud/internal/repository"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	db     *db.Database
}

// NewServer creates a new server instance
func NewServer(db *db.Database) *Server {
	// Set release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Disable Gin's default logger entirely because we're using our custom logger
	gin.DisableConsoleColor()
	gin.DefaultWriter = io.Discard

	// Create a new engine without default middleware
	router := gin.New()

	// Always add recovery middleware for panic handling
	router.Use(gin.Recovery())

	return &Server{
		router: router,
		db:     db,
	}
}

// Start starts the server
func (s *Server) Start() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create rate limiter configuration
	rateLimitConfig := middleware.RateLimitConfig{
		RPS:   10, // 10 requests per second
		Burst: 20, // Allow bursts of up to 20 requests
	}

	// Create repositories
	userRepo := repository.NewUserRepository(s.db.DB)
	authRepo := repository.NewAuthRepository(s.db.DB)
	sessionRepo := repository.NewSessionRepository(s.db.DB)

	// Create validation middleware
	validationMiddleware := middleware.NewValidationMiddleware()
	authMiddleware := middleware.NewAuthMiddleware()

	// Create handlers
	authHandler := handlers.NewAuthHandler(authRepo)
	userHandler := handlers.NewUserHandler(userRepo)
	healthHandler := handlers.NewHealthHandler(s.db.DB)
	sessionHandler := handlers.NewSessionHandler(sessionRepo)

	// Add global middleware
	s.router.Use(middleware.CORS())
	s.router.Use(middleware.PreserveRequestBody())
	s.router.Use(middleware.RateLimitMiddleware(rateLimitConfig))
	s.router.Use(middleware.RequestLogger())

	// Health check endpoint - no auth required
	s.router.GET("/health", healthHandler.Check)

	// Public routes
	public := s.router.Group("/api/v1")
	{
		// Auth routes with validation
		public.POST("/auth/register", validationMiddleware.ValidateRegisterRequest(), authHandler.Register)
		public.POST("/auth/login", validationMiddleware.ValidateLoginRequest(), authHandler.Login)
		public.POST("/auth/logout", authHandler.Logout)
		public.GET("/auth/session", authHandler.GetSession)
		public.POST("/auth/verify-token", validationMiddleware.ValidateVerifyTokenRequest(), authHandler.VerifyToken)
	}

	// Protected routes
	protected := s.router.Group("/api/v1")
	protected.Use(authMiddleware.RequireAuth())

	// User routes
	protected.GET("/users", userHandler.ListUsers)
	protected.GET("/users/:id", userHandler.GetUser)
	protected.PUT("/users/:id", validationMiddleware.ValidateUpdateUserRequest(), userHandler.UpdateUser)
	protected.DELETE("/users/:id", userHandler.DeleteUser)
	protected.GET("/user/profile", userHandler.GetProfile)
	protected.PUT("/user/profile", validationMiddleware.ValidateUpdateProfileRequest(), userHandler.UpdateProfile)
	protected.DELETE("/user/profile", userHandler.DeleteProfile)

	// Session routes
	protected.GET("/sessions", sessionHandler.GetSessions)
	protected.DELETE("/sessions/:id", sessionHandler.RevokeSession)
	protected.DELETE("/sessions", sessionHandler.RevokeAllSessions)

	return s.router.Run(":" + port)
}