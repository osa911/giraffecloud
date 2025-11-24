package db

import (
	"context"
	"fmt"
	"os"

	"github.com/osa911/giraffecloud/internal/db/ent"

	"entgo.io/ent/dialect"

	_ "github.com/lib/pq"
)

// Client represents the database client
var Client *ent.Client

// Initialize sets up the Ent client
func Initialize(dbURL string) (*ent.Client, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL cannot be empty")
	}

	// Create an Ent client
	client, err := ent.Open(dialect.Postgres, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Run the auto migration tool
	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, fmt.Errorf("failed creating schema resources: %v", err)
	}

	Client = client
	return client, nil
}

// Database represents the database connection
type Database struct {
	DB *ent.Client
}

// NewDatabase creates a new database instance
func NewDatabase(db *ent.Client) *Database {
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
// It first checks for DATABASE_URL, and if not present, falls back to individual parameters
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

// ToURL converts the config to a PostgreSQL connection URL
func (c *Config) ToURL() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// GetDatabaseURL returns DATABASE_URL if set, otherwise generates it from individual parameters
func GetDatabaseURL() string {
	// Check if DATABASE_URL is set
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}

	// Fall back to generating URL from individual parameters
	config := NewConfig()
	return config.ToURL()
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
