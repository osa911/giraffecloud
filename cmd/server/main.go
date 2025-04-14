package main

import (
	"log"
	"os"

	"giraffecloud/internal/config"
	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/server"
	"giraffecloud/internal/tasks"
)

func main() {
	// Initialize logger
	logConfig := &config.LoggingConfig{
		Level:      os.Getenv("LOG_LEVEL"),
		File:       os.Getenv("LOG_FILE"),
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	}

	// Use default values if not set
	if logConfig.Level == "" {
		logConfig.Level = "info"
	}
	if logConfig.File == "" {
		logConfig.File = "~/.giraffecloud/api.log"
	}

	logger, err := logging.NewLogger(logConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

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
	srv := server.NewServer(database)

	// Start server
	if err := srv.Start(); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}