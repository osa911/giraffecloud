package main

import (
	"os"

	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/server"
	"giraffecloud/internal/tasks"
)

func main() {
	// Set development environment variables
	if os.Getenv("ENV") != "production" {
		os.Setenv("ENV", "development")
	}

	// Initialize logger configuration
	logConfig := &logging.LogConfig{
		File:       os.Getenv("LOG_FILE"),
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	}

	// Use default values if not set
	if logConfig.File == "" {
		logConfig.File = "./logs/api.log"
	}

	// Configure and get logger
	if err := logging.InitLogger(logConfig); err != nil {
		panic(err)
	}
	logger := logging.GetGlobalLogger()
	defer logger.Close()

	logger.Info("Starting server in %s mode", os.Getenv("ENV"))

	// Initialize database connection
	entClient, err := db.Initialize()
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		os.Exit(1)
	}
	defer entClient.Close()

	// Create database wrapper
	database := db.NewDatabase(entClient)

	// Initialize Firebase
	if err := firebase.InitializeFirebase(); err != nil {
		logger.Error("Failed to initialize Firebase: %v", err)
		os.Exit(1)
	}

	// Start session cleanup task
	sessionCleanup := tasks.NewSessionCleanup(entClient)
	sessionCleanup.Start()
	logger.Info("Started session cleanup task")

	// Initialize server
	cfg := &server.Config{
		Port: os.Getenv("PORT"),
	}

	// Use default values if not set
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	// Create and start server
	srv, err := server.NewServer(database)
	if err != nil {
		logger.Error("Failed to create server: %v", err)
		os.Exit(1)
	}

	// Initialize server
	if err := srv.Init(); err != nil {
		logger.Error("Failed to initialize server: %v", err)
		os.Exit(1)
	}

	if err := srv.Start(cfg); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}