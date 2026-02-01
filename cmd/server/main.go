package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/osa911/giraffecloud/internal/config"
	"github.com/osa911/giraffecloud/internal/config/firebase"
	"github.com/osa911/giraffecloud/internal/db"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/server"
	"github.com/osa911/giraffecloud/internal/tasks"
	"github.com/osa911/giraffecloud/internal/telemetry"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered: %v\nStack trace:\n%s\n", r, debug.Stack())
			os.Exit(1)
		}
	}()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger configuration
	logConfig := &logging.LogConfig{
		File:       cfg.LogFile,
		MaxSize:    100,          // 100MB
		MaxBackups: 10,           // Keep 10 backups
		MaxAge:     30,           // Keep logs for 30 days
		Level:      cfg.LogLevel, // Default to empty string, will use INFO level
		Format:     cfg.LogFormat,
	}

	// Configure and get logger
	if err := logging.InitLogger(logConfig); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	logger := logging.GetGlobalLogger()
	defer logger.Close()

	logger.Info("Starting server in %s mode", cfg.Environment)
	logger.Info("Log file location: %s", cfg.LogFile)

	// Log critical configuration values for DNS polling
	if cfg.ServerIP != "" {
		logger.Info("Server IP configured: %s", cfg.ServerIP)
	} else {
		logger.Warn("SERVER_IP not configured - DNS monitor will be disabled")
	}
	logger.Info("Base domain: %s", cfg.BaseDomain)

	// Initialize Tracing
	if cfg.OTLPEndpoint != "" {
		logger.Info("Initializing OpenTelemetry tracing...")
		shutdown, err := telemetry.InitTracer(context.Background(), "giraffecloud-api", cfg.OTLPEndpoint)
		if err != nil {
			logger.Error("Failed to initialize tracing: %v", err)
		} else {
			defer func() {
				if err := shutdown(context.Background()); err != nil {
					logger.Error("Failed to shutdown tracer: %v", err)
				}
			}()
			logger.Info("OpenTelemetry tracing initialized")
		}
	}

	// Initialize database connection
	logger.Info("Initializing database connection...")
	dbURL := db.GetDatabaseURL()
	entClient, err := db.Initialize(dbURL)
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

	// Create and start server
	logger.Info("Creating server instance...")
	srv, err := server.NewServer(cfg, database)
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
	if err := srv.Start(); err != nil {
		logger.Error("Server failed to start: %v\nStack trace:\n%s", err, debug.Stack())
		os.Exit(1)
	}
}
