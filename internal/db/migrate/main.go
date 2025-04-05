package main

import (
	"fmt"
	"os"

	"giraffecloud/internal/config/env"
	"giraffecloud/internal/db"
	"giraffecloud/internal/models"
)

func main() {
	// Load environment variables
	if err := env.LoadEnv(); err != nil {
		fmt.Printf("Failed to load environment: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.Initialize()
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}

	// Run migrations
	if err := database.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamUser{},
		&models.Tunnel{},
		&models.Session{},
	); err != nil {
		fmt.Printf("Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations completed successfully")
}