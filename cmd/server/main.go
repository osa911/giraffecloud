package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/server"
	"giraffecloud/internal/tasks"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered: %v\nStack trace:\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	// Set development environment variables
	if os.Getenv("ENV") != "production" {
		os.Setenv("ENV", "development")
	}

	// Initialize logger configuration
	logConfig := &logging.LogConfig{
		File:       os.Getenv("LOG_FILE"),
		MaxSize:    100,    // 100MB
		MaxBackups: 10,     // Keep 10 backups
		MaxAge:     30,     // Keep logs for 30 days
	}

	// Use environment-specific log file paths
	if logConfig.File == "" {
		if os.Getenv("ENV") == "production" {
			logConfig.File = "/app/logs/api.log"  // Docker-friendly path
		} else {
			logConfig.File = "./logs/api.log"
		}
	}

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(logConfig.File), 0755); err != nil {
		panic(fmt.Sprintf("Failed to create log directory: %v", err))
	}

	// Configure and get logger
	if err := logging.InitLogger(logConfig); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	logger := logging.GetGlobalLogger()
	defer logger.Close()

	logger.Info("Starting server in %s mode", os.Getenv("ENV"))
	logger.Info("Log file location: %s", logConfig.File)

	// Initialize database connection
	logger.Info("Initializing database connection...")
	entClient, err := db.Initialize()
	if err != nil {
		logger.Error("Failed to initialize database: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
	defer entClient.Close()
	logger.Info("Database connection established successfully")

	// Create database wrapper
	database := db.NewDatabase(entClient)
	logger.Info("Database wrapper created")

	// Initialize Firebase
	logger.Info("Initializing Firebase...")
	if err := firebase.InitializeFirebase(); err != nil {
		logger.Error("Failed to initialize Firebase: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
	logger.Info("Firebase initialized successfully")

	// Start session cleanup task
	logger.Info("Starting session cleanup task...")
	sessionCleanup := tasks.NewSessionCleanup(entClient)
	sessionCleanup.Start()
	defer sessionCleanup.Stop()
	logger.Info("Session cleanup task started successfully")

	// Initialize server
	cfg := &server.Config{
		Port: os.Getenv("PORT"),
	}

	// Use default values if not set
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	// Log important environment variables
	logger.Info("Server configuration:")
	logger.Info("- Port: %s", cfg.Port)
	logger.Info("- Environment: %s", os.Getenv("ENV"))
	logger.Info("- Database URL: %s", os.Getenv("DATABASE_URL"))
	logger.Info("- Log File: %s", logConfig.File)
	logger.Info("- Caddy Admin API: %s", os.Getenv("CADDY_ADMIN_API"))

	// Create and start server
	logger.Info("Creating server instance...")
	srv, err := server.NewServer(database)
	if err != nil {
		logger.Error("Failed to create server: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
	logger.Info("Server instance created successfully")

	// Initialize server
	logger.Info("Initializing server...")
	if err := srv.Init(); err != nil {
		logger.Error("Failed to initialize server: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
	logger.Info("Server initialized successfully")

	logger.Info("Starting server on port %s...", cfg.Port)
	if err := srv.Start(cfg); err != nil {
		logger.Error("Server failed to start: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
}