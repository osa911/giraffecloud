package db

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Database represents the database connection
type Database struct {
	DB *gorm.DB
}

// NewDatabase creates a new database instance
func NewDatabase(db *gorm.DB) *Database {
	return &Database{
		DB: db,
	}
}

// Config represents database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfig creates a new database configuration from environment variables
func NewConfig() *Config {
	return &Config{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvOrDefaultInt("DB_PORT", 5432),
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("DB_NAME", "db_name"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
	}
}

// Initialize sets up the database connection
func Initialize() (*gorm.DB, error) {
	env := getEnvOrDefault("ENV", "development")

	// Debug log environment variables
	fmt.Printf("==== Initializing Database ====\n")
	fmt.Printf("Database environment variables:\n")
	fmt.Printf("DB_HOST: %s\n", os.Getenv("DB_HOST"))
	fmt.Printf("DB_PORT: %s\n", os.Getenv("DB_PORT"))
	fmt.Printf("DB_USER: %s\n", os.Getenv("DB_USER"))
	fmt.Printf("DB_NAME: %s\n", os.Getenv("DB_NAME"))
	fmt.Printf("DB_SSL_MODE: %s\n", os.Getenv("DB_SSL_MODE"))

	// Try to use DATABASE_URL first
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Fall back to individual environment variables
		config := NewConfig()

		// Verify that we have all required parameters
		if config.DBName == "" {
			return nil, fmt.Errorf("database name is required, DB_NAME environment variable is not set")
		}

		dbURL = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
		)

		// Log the connection string (with password masked)
		logURL := fmt.Sprintf(
			"host=%s port=%d user=%s password=*** dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.DBName, config.SSLMode,
		)
		fmt.Printf("Database connection string: %s\n", logURL)
	}

	// Configure GORM logger based on environment
	logLevel := logger.Info
	if env == "production" {
		logLevel = logger.Silent
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	DB = db
	return db, nil
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}