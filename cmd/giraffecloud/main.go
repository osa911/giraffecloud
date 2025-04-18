package main

import (
	"fmt"
	"giraffecloud/internal/config"
	"giraffecloud/internal/logging"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "giraffecloud",
	Short: "GiraffeCloud CLI - Secure reverse tunnel client",
	Long: `GiraffeCloud CLI is a secure reverse tunnel client that allows you to expose
local services through GiraffeCloud's infrastructure.`,
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to GiraffeCloud and establish a tunnel",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Configure and get logger
		logging.Configure(&cfg.Logging)
		logger := logging.GetLogger()

		logger.Info("Starting GiraffeCloud tunnel")
		logger.Debug("Configuration loaded: %+v", cfg)

		t := tunnel.NewTunnel(cfg)
		if err := t.Connect(); err != nil {
			logger.Error("Failed to connect to GiraffeCloud: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully connected to GiraffeCloud")

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Wait for signal
		sig := <-sigChan
		logger.Info("Received signal %v, shutting down...", sig)

		// Graceful shutdown
		if err := t.Disconnect(); err != nil {
			logger.Error("Error during shutdown: %v", err)
			os.Exit(1)
		}

		logger.Info("Shutdown complete")
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage GiraffeCloud service",
	Long:  `Install, uninstall, or manage the GiraffeCloud service on your system.`,
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install GiraffeCloud as a system service",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Configure and get logger
		logging.Configure(&cfg.Logging)
		logger := logging.GetLogger()

		logger.Info("Installing GiraffeCloud service")

		sm, err := tunnel.NewServiceManager()
		if err != nil {
			logger.Error("Failed to create service manager: %v", err)
			os.Exit(1)
		}

		if err := sm.Install(); err != nil {
			logger.Error("Failed to install service: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully installed GiraffeCloud service")
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall GiraffeCloud system service",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Configure and get logger
		logging.Configure(&cfg.Logging)
		logger := logging.GetLogger()

		logger.Info("Uninstalling GiraffeCloud service")

		sm, err := tunnel.NewServiceManager()
		if err != nil {
			logger.Error("Failed to create service manager: %v", err)
			os.Exit(1)
		}

		if err := sm.Uninstall(); err != nil {
			logger.Error("Failed to uninstall service: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully uninstalled GiraffeCloud service")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Info())
	},
}

type Config struct {
	Token string `yaml:"token"`
}

func saveConfig(cfg *Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".giraffecloud")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func loadConfig() (*Config, error) {
	configFile := filepath.Join(os.Getenv("HOME"), ".giraffecloud", "config.yaml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

func init() {
	rootCmd.AddCommand(connectCmd)

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)

	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(versionCmd)

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Login to GiraffeCloud using an API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := cmd.Flags().GetString("token")
			if err != nil {
				return fmt.Errorf("failed to get token flag: %w", err)
			}

			if token == "" {
				return fmt.Errorf("token is required")
			}

			cfg := &Config{
				Token: token,
			}

			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("Successfully logged in to GiraffeCloud")
			return nil
		},
	}

	loginCmd.Flags().String("token", "", "API token for authentication")
	loginCmd.MarkFlagRequired("token")

	rootCmd.AddCommand(loginCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}