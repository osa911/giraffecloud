package main

import (
	"fmt"
	"giraffecloud/internal/config"
	"giraffecloud/internal/logging"
	"os"
	"os/signal"
	"syscall"

	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"

	"github.com/spf13/cobra"
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

var loginCmd = &cobra.Command{
	Use:   "login --token <TOKEN>",
	Short: "Login to GiraffeCloud using an API token",
	Long: `Login to GiraffeCloud using an API token.
The token will be stored securely in your config file (~/.giraffecloud/config).
Example: giraffecloud login --token your-api-token`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := cmd.Flags().GetString("token")
		if err != nil {
			return fmt.Errorf("failed to get token flag: %w", err)
		}

		// Load existing config or use defaults
		cfg, err := config.LoadConfig()
		if err != nil {
			// If config doesn't exist, start with defaults
			cfg = &config.DefaultConfig
		}

		// Update token while preserving other settings
		cfg.Token = token

		// Validate the config
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("invalid configuration: %w", err)
		}

		// Save the updated config
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		fmt.Println("Successfully logged in to GiraffeCloud")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(loginCmd)

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)

	loginCmd.Flags().String("token", "", "API token for authentication")
	loginCmd.MarkFlagRequired("token")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}