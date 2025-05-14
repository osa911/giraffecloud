package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"giraffecloud/internal/api/handlers"
	"giraffecloud/internal/api/middleware"
	"giraffecloud/internal/caddy"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/repository"
	"giraffecloud/internal/server/routes"
	"giraffecloud/internal/service"
	"giraffecloud/internal/tunnel"

	"github.com/gin-gonic/gin"
)

// serverManager handles the lifecycle of HTTP and tunnel servers
type serverManager struct {
	httpServer   *http.Server
	tunnelServer *tunnel.TunnelServer
}

// newServerManager creates a new server manager
func newServerManager(router *gin.Engine, tunnelSrv *tunnel.TunnelServer, port string) *serverManager {
	// Configure http.Server with environment-specific settings
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Production settings
	if os.Getenv("ENV") == "production" {
		httpServer.ReadTimeout = 15 * time.Second
		httpServer.WriteTimeout = 15 * time.Second
		httpServer.IdleTimeout = 60 * time.Second
		httpServer.ReadHeaderTimeout = 5 * time.Second
		httpServer.MaxHeaderBytes = 1 << 20 // 1MB
	} else {
		// Development settings - more lenient timeouts for debugging
		httpServer.ReadTimeout = 120 * time.Second    // Longer timeout for debugging
		httpServer.WriteTimeout = 120 * time.Second   // Longer timeout for debugging
		httpServer.IdleTimeout = 180 * time.Second    // Longer idle timeout
		httpServer.ReadHeaderTimeout = 30 * time.Second
		httpServer.MaxHeaderBytes = 1 << 20 // 1MB
	}

	return &serverManager{
		httpServer:   httpServer,
		tunnelServer: tunnelSrv,
	}
}

// start starts both HTTP and tunnel servers
func (sm *serverManager) start() error {
	logger := logging.GetGlobalLogger()

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP server on port %s", sm.httpServer.Addr)
		if err := sm.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	return sm.shutdown()
}

// shutdown gracefully shuts down both servers
func (sm *serverManager) shutdown() error {
	logger := logging.GetGlobalLogger()

	logger.Info("Shutting down servers...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := sm.httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	}

	// Shutdown tunnel server
	if sm.tunnelServer != nil {
		if err := sm.tunnelServer.Stop(); err != nil {
			logger.Error("Tunnel server shutdown error: %v", err)
		}
	}

	logger.Info("Servers shutdown complete")
	return nil
}

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

	// Initialize connection manager
	logger.Info("Initializing connection manager...")
	connectionManager := tunnel.NewConnectionManager()
	logger.Info("Connection manager initialized")

	// Initialize Caddy service if in production
	var caddyService service.CaddyService
	if os.Getenv("ENV") == "production" {
		// Initialize Caddy service using standardized configuration
		logger.Info("Initializing Caddy service...")
		logger.Info("Using Caddy config path: %s", caddy.CaddyPaths.Config)

		caddyService = service.NewCaddyService(connectionManager)

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

	// Initialize tunnel server
	logger.Info("Initializing tunnel server...")
	s.tunnelServer = tunnel.NewServer(repos.Token, repos.Tunnel, tunnelService)

	tunnelPort := os.Getenv("TUNNEL_PORT")
	if tunnelPort == "" {
		tunnelPort = "4443"
	}
	if err := s.tunnelServer.Start(":" + tunnelPort); err != nil {
		logger.Error("Failed to start tunnel server: %v", err)
		return fmt.Errorf("failed to start tunnel server: %w", err)
	}
	logger.Info("Tunnel server started on port %s", tunnelPort)

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

	// Log configuration
	logger.Info("Server configuration:")
	logger.Info("- HTTP Port: %s", cfg.Port)
	logger.Info("- Environment: %s", os.Getenv("ENV"))
	logger.Info("- Database URL: %s", os.Getenv("DATABASE_URL"))
	logger.Info("- Tunnel Port: %s", os.Getenv("TUNNEL_PORT"))
	if os.Getenv("ENV") == "production" {
		logger.Info("- Caddy Config: %s", caddy.CaddyPaths.Config)
	}

	// Custom handler for tunnel domains only
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := r.Host
		logger.Info("[HIJACK DEBUG] Received HTTP request for domain: %s, method: %s, path: %s", domain, r.Method, r.URL.Path)
		logger.Info("[HIJACK DEBUG] Headers: %+v", r.Header)

		isTunnel := s.tunnelServer.IsTunnelDomain(domain)
		logger.Info("[HIJACK DEBUG] HTTP request for domain: %s, isTunnel: %v", domain, isTunnel)

		if isTunnel {
			logger.Info("[HIJACK DEBUG] Attempting to hijack connection for domain: %s", domain)
			hj, ok := w.(http.Hijacker)
			if !ok {
				logger.Error("[HIJACK DEBUG] Failed to hijack: ResponseWriter doesn't support hijacking")
				http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
				return
			}

			// Write the initial response headers before hijacking
			w.Header().Set("Connection", "upgrade")
			w.Header().Set("Upgrade", "tcp")
			w.WriteHeader(http.StatusSwitchingProtocols)

			conn, bufrw, err := hj.Hijack()
			if err != nil {
				logger.Error("[HIJACK DEBUG] Hijack failed: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer conn.Close()
			logger.Info("[HIJACK DEBUG] Successfully hijacked connection for domain: %s", domain)

			// Check if there's any buffered data
			buffered := bufrw.Reader.Buffered()
			if buffered > 0 {
				logger.Info("[HIJACK DEBUG] Found %d bytes of buffered data in hijacked connection", buffered)
				data := make([]byte, buffered)
				n, err := bufrw.Reader.Read(data)
				if err != nil {
					logger.Error("[HIJACK DEBUG] Error reading buffered data: %v", err)
					return
				}
				logger.Info("[HIJACK DEBUG] Read %d bytes of buffered data: %s", n, string(data[:n]))
			}

			logger.Info("[HIJACK DEBUG] Proxying connection for domain: %s", domain)
			s.tunnelServer.ProxyConnection(domain, conn)
			return
		}

		logger.Info("[HIJACK DEBUG] Not a tunnel domain, forwarding to Gin router: %s", domain)
		s.router.ServeHTTP(w, r)
	})

	// Start HTTP server on :8081 for Caddy to forward tunnel domain requests
	go func() {
		logger.Info("Starting HTTP hijack server on :8081 for tunnel domains")
		server := &http.Server{
			Addr:    ":8081",
			Handler: httpHandler,
			// Add timeouts to prevent hanging connections
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    60 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP hijack server error: %v", err)
		}
		logger.Info("HTTP hijack server stopped")
	}()

	// Create server manager for the main Gin HTTP API (if needed)
	manager := newServerManager(s.router, s.tunnelServer, cfg.Port)

	// Start servers
	return manager.start()
}

// generateDevCertificate generates a self-signed certificate for development
func (s *Server) generateDevCertificate(certFile, keyFile string) error {
	logger := logging.GetGlobalLogger()

	// Create certs directory if it doesn't exist
	if err := os.MkdirAll("certs", 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Check if certificates already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			logger.Info("Development certificates already exist")
			return nil
		}
	}

	// Generate certificate using openssl
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:4096", "-keyout", keyFile,
		"-out", certFile, "-days", "365", "-nodes", "-subj",
		"/C=US/ST=Development/L=Development/O=GiraffeCloud Dev/CN=tunnel.giraffecloud.xyz")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate certificate: %w\nOutput: %s", err, output)
	}

	logger.Info("Generated development certificates successfully")
	return nil
}