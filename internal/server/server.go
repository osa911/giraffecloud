package server

import (
	"os"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/db"

	"github.com/gin-gonic/gin"
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

	// Create rate limiter configuration
	rateLimitConfig := middleware.RateLimitConfig{
		RPS:   10, // 10 requests per second
		Burst: 20, // Allow bursts of up to 20 requests
	}

	// Create validation middleware
	validationMiddleware := middleware.NewValidationMiddleware()
	authMiddleware := middleware.NewAuthMiddleware()

	// Create handlers
	authHandler := handlers.NewAuthHandler(s.db.DB)
	userHandler := handlers.NewUserHandler(s.db.DB)

	// Add global middleware
	s.router.Use(middleware.CORS())
	s.router.Use(middleware.PreserveRequestBody())
	s.router.Use(middleware.RateLimitMiddleware(rateLimitConfig))

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
	protected.Use(authMiddleware.RequireAuth())
	{
		// User routes with validation
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.PUT("/user/profile", validationMiddleware.ValidateUpdateProfileRequest(), userHandler.UpdateProfile)
		protected.DELETE("/user/profile", userHandler.DeleteProfile)

		// Create tunnel handler
		tunnelHandler := handlers.NewTunnelHandler(s.db.DB)

		// Tunnel routes with validation
		protected.GET("/tunnels", tunnelHandler.ListTunnels)
		protected.POST("/tunnels", validationMiddleware.ValidateCreateTunnelRequest(), tunnelHandler.CreateTunnel)
		protected.GET("/tunnels/:id", tunnelHandler.GetTunnel)
		protected.PUT("/tunnels/:id", validationMiddleware.ValidateUpdateTunnelRequest(), tunnelHandler.UpdateTunnel)
		protected.DELETE("/tunnels/:id", tunnelHandler.DeleteTunnel)
		protected.POST("/tunnels/:id/start", tunnelHandler.StartTunnel)
		protected.POST("/tunnels/:id/stop", tunnelHandler.StopTunnel)

		// Admin routes
		admin := protected.Group("/admin")
		admin.Use(authMiddleware.RequireAdmin())
		{
			admin.GET("/users", userHandler.ListUsers)
			admin.GET("/users/:id", userHandler.GetUser)
			admin.PUT("/users/:id", validationMiddleware.ValidateUpdateUserRequest(), userHandler.UpdateUser)
			admin.DELETE("/users/:id", userHandler.DeleteUser)
		}
	}

	return s.router.Run(":" + port)
}