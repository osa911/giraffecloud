package main

import (
	"log"

	"giraffecloud/internal/config/firebase"
	"giraffecloud/internal/db"
	"giraffecloud/internal/server"
	"giraffecloud/internal/tasks"
)

func main() {
	// Initialize database connection
	gormDB, err := db.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create database instance
	database := db.NewDatabase(gormDB)

	// Initialize Firebase
	if err := firebase.InitializeFirebase(); err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}

	// Start session cleanup task
	sessionCleanup := tasks.NewSessionCleanup(database)
	sessionCleanup.Start()
	log.Println("Started session cleanup task")

	// Initialize server
	srv := server.NewServer(database)

	// Start server
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}