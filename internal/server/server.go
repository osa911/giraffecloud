package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
	tunnelRouter *tunnel.HybridTunnelRouter // Changed from TunnelServer to HybridTunnelRouter
}

// newServerManager creates a new server manager
func newServerManager(router *gin.Engine, tunnelRouter *tunnel.HybridTunnelRouter, port string) *serverManager {
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
		httpServer.ReadTimeout = 120 * time.Second  // Longer timeout for debugging
		httpServer.WriteTimeout = 120 * time.Second // Longer timeout for debugging
		httpServer.IdleTimeout = 180 * time.Second  // Longer idle timeout
		httpServer.ReadHeaderTimeout = 30 * time.Second
		httpServer.MaxHeaderBytes = 1 << 20 // 1MB
	}

	return &serverManager{
		httpServer:   httpServer,
		tunnelRouter: tunnelRouter, // Changed from tunnelServer
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

	// Shutdown tunnel router
	if sm.tunnelRouter != nil {
		if err := sm.tunnelRouter.Stop(); err != nil {
			logger.Error("Tunnel router shutdown error: %v", err)
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
	versionService := service.NewVersionService(s.db.DB)
	planService := service.NewPlanService(s.db.DB)
	usageService := service.NewUsageServiceWithDB(s.db.DB)
	quotaService := service.NewQuotaService(s.db.DB)
	logger.Info("Core services initialized")

	// Initialize default version configurations
	if err := versionService.InitializeDefaultConfigs(context.Background()); err != nil {
		logger.Warn("Failed to initialize default version configs: %v", err)
	}

	// Seed default plans (idempotent)
	if err := planService.SeedDefaultPlans(context.Background()); err != nil {
		logger.Warn("Failed to seed default plans: %v", err)
	}

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

	// Initialize Hybrid Tunnel Router (Production-Grade Architecture)
	logger.Info("Initializing Hybrid Tunnel Router...")

	// Create hybrid router configuration
	routerConfig := tunnel.DefaultHybridRouterConfig()

	// Override ports from environment variables
	if grpcPort := os.Getenv("GRPC_TUNNEL_PORT"); grpcPort != "" {
		routerConfig.GRPCAddress = ":" + grpcPort
	} else {
		routerConfig.GRPCAddress = ":4444" // Default gRPC port
	}

	if tcpPort := os.Getenv("TUNNEL_PORT"); tcpPort != "" {
		routerConfig.TCPAddress = ":" + tcpPort
	} else {
		routerConfig.TCPAddress = ":4443" // Default TCP port
	}

	// Create the hybrid tunnel router
	s.tunnelRouter = tunnel.NewHybridTunnelRouter(repos.Token, repos.Tunnel, tunnelService, routerConfig)
	// Wire usage recorder into tunnel router and underlying servers
	s.tunnelRouter.SetUsageRecorder(usageService)
	// Adapt service.QuotaService to tunnel.QuotaChecker
	s.tunnelRouter.SetQuotaChecker(quotaAdapter{q: quotaService})

	// Periodically flush usage to DB (every 1 minute)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := usageService.FlushToDB(context.Background(), s.db.DB); err != nil {
				logger.Warn("Failed to flush usage to DB: %v", err)
			}
		}
	}()

	// Start the hybrid tunnel router
	if err := s.tunnelRouter.Start(); err != nil {
		logger.Error("Failed to start hybrid tunnel router: %v", err)
		return fmt.Errorf("failed to start hybrid tunnel router: %w", err)
	}

	logger.Info("🚀 Hybrid Tunnel Router started successfully!")
	logger.Info("  ✓ gRPC Tunnel (HTTP): %s", routerConfig.GRPCAddress)
	logger.Info("  ✓ TCP Tunnel (WebSocket): %s", routerConfig.TCPAddress)
	logger.Info("  ✓ Production-grade unlimited concurrency enabled")

	// Initialize handlers
	logger.Info("Initializing handlers...")
	handlers := &routes.Handlers{
		Auth:              handlers.NewAuthHandler(repos.Auth, repos.Session, csrfService, auditService),
		User:              handlers.NewUserHandler(repos.User),
		Health:            handlers.NewHealthHandler(s.db.DB),
		Session:           handlers.NewSessionHandler(repos.Session),
		Token:             handlers.NewTokenHandler(tokenService),
		Tunnel:            handlers.NewTunnelHandler(tunnelService, versionService),
		TunnelCertificate: handlers.NewTunnelCertificateHandler(),
		Webhook:           handlers.NewWebhookHandler(),
		Admin:             handlers.NewAdminHandler(versionService),
		Usage:             handlers.NewUsageHandler(s.db.DB, quotaService),
		Contact:           handlers.NewContactHandler(),
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
	logger.Info("- gRPC Tunnel Port: %s", os.Getenv("GRPC_TUNNEL_PORT"))
	logger.Info("- TCP Tunnel Port: %s", os.Getenv("TUNNEL_PORT"))
	if os.Getenv("ENV") == "production" {
		logger.Info("- Caddy Config: %s", caddy.CaddyPaths.Config)
	}

	// Custom handler for tunnel domains only
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := r.Host
		isTunnel := s.tunnelRouter.IsTunnelDomain(domain)

		if isTunnel {
			// Use connection hijacking for all tunnel traffic
			hj, ok := w.(http.Hijacker)
			if !ok {
				logger.Error("Failed to hijack: ResponseWriter doesn't support hijacking")
				http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
				return
			}

			conn, bufrw, err := hj.Hijack()
			if err != nil {
				logger.Error("Failed to hijack connection: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer conn.Close()

			// Ensure any buffered data is flushed before proxying
			if bufrw.Writer.Buffered() > 0 {
				if err := bufrw.Writer.Flush(); err != nil {
					logger.Error("Failed to flush buffered data: %v", err)
					return
				}
			}

			// Build the complete HTTP request string
			var requestData strings.Builder

			// Add request line
			requestData.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, r.URL.RequestURI()))

			// Add Host header first
			requestData.WriteString(fmt.Sprintf("Host: %s\r\n", r.Host))

			// Add Content-Length if body exists
			if r.ContentLength > 0 {
				requestData.WriteString(fmt.Sprintf("Content-Length: %d\r\n", r.ContentLength))
			}

			// Add remaining headers, skipping those we've already handled
			for key, values := range r.Header {
				if key != "Host" && key != "Content-Length" { // Skip headers we've already added
					for _, value := range values {
						requestData.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
					}
				}
			}

			// Add empty line to separate headers from body
			requestData.WriteString("\r\n")

			// Get the request as bytes
			requestBytes := []byte(requestData.String())

			// Let the HybridTunnelRouter intelligently route the traffic
			// It will automatically detect WebSocket upgrades and route to TCP tunnel
			// Regular HTTP requests will be routed to gRPC tunnel for unlimited concurrency
			s.tunnelRouter.ProxyConnection(domain, conn, requestBytes, r.Body)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	})

	// Start HTTP server on :8081 for Caddy to forward tunnel domain requests
	go func() {

		// Get hijack port with default fallback
		hijackPort := os.Getenv("HIJACK_PORT")
		if hijackPort == "" {
			hijackPort = "8081"
		}

		logger.Info("Starting HTTP hijack server on :%s for tunnel domains", hijackPort)

		server := &http.Server{
			Addr:    ":" + hijackPort,
			Handler: httpHandler,
			// Add timeouts to prevent hanging connections
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP hijack server error: %v", err)
		}
		logger.Info("HTTP hijack server stopped")
	}()

	// Create server manager for the main Gin HTTP API (if needed)
	manager := newServerManager(s.router, s.tunnelRouter, cfg.Port)

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

// isWebSocketUpgrade checks if the HTTP request is a WebSocket upgrade request
func isWebSocketUpgrade(r *http.Request) bool {
	// Check for WebSocket upgrade headers
	connection := r.Header.Get("Connection")
	upgrade := r.Header.Get("Upgrade")
	webSocketKey := r.Header.Get("Sec-WebSocket-Key")

	// Connection header should contain "upgrade" (case-insensitive)
	connectionUpgrade := false
	for _, part := range strings.Split(strings.ToLower(connection), ",") {
		if strings.TrimSpace(part) == "upgrade" {
			connectionUpgrade = true
			break
		}
	}

	// Upgrade header should be "websocket" (case-insensitive)
	upgradeWebSocket := strings.ToLower(strings.TrimSpace(upgrade)) == "websocket"

	// Must have Sec-WebSocket-Key header
	hasWebSocketKey := webSocketKey != ""

	return connectionUpgrade && upgradeWebSocket && hasWebSocketKey
}
