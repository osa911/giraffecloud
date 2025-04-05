package server

import (
	"os"
	"time"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/db"

	"github.com/gin-gonic/gin"
	"go.uber.org/ratelimit"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	db     *db.Database
}

// NewServer creates a new server instance
func NewServer(db *db.Database) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Explicitly configure the router for JSON
	router.Use(gin.Logger())
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

	// Create rate limiter
	limiter := ratelimit.New(10, ratelimit.Per(time.Second))

	// Create validation middleware
	validationMiddleware := middleware.NewValidationMiddleware()

	// Create handlers
	authHandler := handlers.NewAuthHandler(s.db.DB)
	userHandler := handlers.NewUserHandler(s.db.DB)

	// Add global middleware
	s.router.Use(middleware.CORS())
	s.router.Use(middleware.RateLimitMiddleware(limiter))

	// Public routes
	public := s.router.Group("/api/v1")
	{
		// Auth routes with validation
		public.POST("/auth/register", validationMiddleware.ValidateRegisterRequest(), authHandler.Register)
		public.POST("/auth/login", validationMiddleware.ValidateLoginRequest(), authHandler.Login)
		public.POST("/auth/logout", authHandler.Logout)
		public.GET("/auth/session", authHandler.GetSession)
	}

	// Protected routes
	protected := s.router.Group("/api/v1")
	protected.Use(middleware.RequireAuth())
	{
		// User routes with validation
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.PUT("/user/profile", validationMiddleware.ValidateUpdateProfileRequest(), userHandler.UpdateProfile)
		protected.DELETE("/user/profile", userHandler.DeleteProfile)

		// Admin routes
		admin := protected.Group("/admin")
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/users", userHandler.ListUsers)
			admin.GET("/users/:id", userHandler.GetUser)
			admin.PUT("/users/:id", userHandler.UpdateUser)
			admin.DELETE("/users/:id", userHandler.DeleteUser)
		}
	}

	return s.router.Run(":" + port)
}