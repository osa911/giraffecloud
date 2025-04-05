package env

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from the appropriate .env file
func LoadEnv() error {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	envFile := fmt.Sprintf(".env.%s", env)
	envPath := filepath.Join("internal", "config", "env", envFile)

	fmt.Printf("Loading environment from: %s\n", envPath)

	// Load env file
	if err := godotenv.Load(envPath); err != nil {
		return fmt.Errorf("error loading env file %s: %v", envPath, err)
	}

	// Verify that critical variables are loaded
	requiredVars := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			fmt.Printf("Warning: Required environment variable %s is not set or empty\n", v)
		} else {
			fmt.Printf("Loaded: %s=%s\n", v, os.Getenv(v))
		}
	}

	return nil
}