package api

import (
	"time"

	"giraffecloud/internal/middleware"

	"giraffecloud/internal/api/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	router *gin.Engine
	db     *gorm.DB
}

func NewServer(db *gorm.DB) *Server {
	server := &Server{
		router: gin.Default(),
		db:     db,
	}

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"}
	config.MaxAge = 12 * time.Hour
	server.router.Use(cors.New(config))

	// Add middleware
	server.router.Use(middleware.Recovery())
	server.router.Use(middleware.RequestID())
	server.router.Use(middleware.Logger())

	// Initialize routes
	server.initializeRoutes()

	return server
}

func (s *Server) initializeRoutes() {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(s.db)
	userHandler := handlers.NewUserHandler(s.db)
	teamHandler := handlers.NewTeamHandler(s.db)
	tunnelHandler := handlers.NewTunnelHandler(s.db)
	sessionHandler := handlers.NewSessionHandler(s.db)
	adminHandler := handlers.NewAdminHandler(s.db)

	// Public routes
	public := s.router.Group("/api/v1")
	{
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/register", authHandler.Register)
	}

	// Protected routes
	auth := middleware.NewAuthMiddleware(s.db)
	protected := s.router.Group("/api/v1")
	protected.Use(auth.RequireAuth())
	{
		// User routes
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.PUT("/user/profile", userHandler.UpdateProfile)
		protected.POST("/user/logout", authHandler.Logout)

		// Team routes
		protected.GET("/teams", teamHandler.ListTeams)
		protected.POST("/teams", teamHandler.CreateTeam)
		protected.GET("/teams/:id", teamHandler.GetTeam)
		protected.PUT("/teams/:id", teamHandler.UpdateTeam)
		protected.DELETE("/teams/:id", teamHandler.DeleteTeam)
		protected.POST("/teams/:id/members", teamHandler.AddTeamMember)
		protected.DELETE("/teams/:id/members/:userId", teamHandler.RemoveTeamMember)

		// Tunnel routes
		protected.GET("/tunnels", tunnelHandler.ListTunnels)
		protected.POST("/tunnels", tunnelHandler.CreateTunnel)
		protected.GET("/tunnels/:id", tunnelHandler.GetTunnel)
		protected.PUT("/tunnels/:id", tunnelHandler.UpdateTunnel)
		protected.DELETE("/tunnels/:id", tunnelHandler.DeleteTunnel)
		protected.POST("/tunnels/:id/start", tunnelHandler.StartTunnel)
		protected.POST("/tunnels/:id/stop", tunnelHandler.StopTunnel)

		// Session routes
		protected.GET("/sessions", sessionHandler.ListSessions)
		protected.DELETE("/sessions/:id", sessionHandler.RevokeSession)
	}

	// Admin routes
	admin := protected.Group("/admin")
	admin.Use(auth.RequireAdmin())
	{
		admin.GET("/users", adminHandler.ListUsers)
		admin.GET("/users/:id", adminHandler.GetUser)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)
	}
}

func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}