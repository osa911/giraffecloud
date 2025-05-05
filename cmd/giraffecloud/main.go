package main

import (
	"crypto/tls"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var logger *logging.Logger

func initLogger() {
	// Initialize logger configuration
	logConfig := &logging.LogConfig{
		File:       "~/.giraffecloud/client.log",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     7,
	}

	// Initialize the global logger
	if err := logging.InitLogger(logConfig); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Get the logger instance
	logger = logging.GetGlobalLogger()
}

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
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Get server host from flag if provided
		host, _ := cmd.Flags().GetString("host")
		if host != "" {
			cfg.Server.Host = host
		}

		// Create TLS config
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Security.InsecureSkipVerify,
		}

		// Create and connect tunnel
		t := tunnel.NewTunnel()
		serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := t.Connect(serverAddr, cfg.Token, tlsConfig); err != nil {
			logger.Error("Failed to connect to GiraffeCloud: %v", err)
			os.Exit(1)
		}

		logger.Info("Connected to GiraffeCloud at %s", serverAddr)
		logger.Info("Tunnel is running. Press Ctrl+C to stop.")

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Wait for signal
		sig := <-sigChan
		logger.Info("Received signal %v, initiating shutdown...", sig)

		// Graceful shutdown
		if err := t.Disconnect(); err != nil {
			logger.Error("Failed to disconnect tunnel: %v", err)
			os.Exit(1)
		}

		logger.Info("Successfully disconnected from GiraffeCloud")
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
		logger.Info("GiraffeCloud version: %s", version.Info())
	},
}

var loginCmd = &cobra.Command{
	Use:   "login --token <TOKEN>",
	Short: "Login to GiraffeCloud using an API token",
	Long: `Login to GiraffeCloud using an API token.
The token will be stored securely in your config file (~/.giraffecloud/config).

After successful login, use 'giraffecloud connect' to establish a tunnel connection.

Example:
  1. giraffecloud login --token your-api-token
  2. giraffecloud connect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := cmd.Flags().GetString("token")
		if err != nil {
			logger.Error("Failed to get token flag: %v", err)
			return err
		}

		// Load existing config or use defaults
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			return err
		}

		// Update token while preserving other settings
		cfg.Token = token

		// Get server host from flag if provided
		host, _ := cmd.Flags().GetString("host")
		if host != "" {
			cfg.Server.Host = host
		}

		// Save the updated config
		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			return err
		}

		logger.Info("Successfully logged in to GiraffeCloud (server: %s)", cfg.Server.Host)
		logger.Info("Run 'giraffecloud connect' to establish a tunnel connection")
		return nil
	},
}

func init() {
	// Initialize logger first
	initLogger()
	logger.Info("Initializing GiraffeCloud CLI")

	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(loginCmd)

	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)

	// Add host flag to connect command
	connectCmd.Flags().String("host", "", "Server host to connect to (default: tunnel.giraffecloud.xyz)")

	// Add host flag to login command
	loginCmd.Flags().String("host", "", "Server host to connect to (default: api.giraffecloud.xyz)")
	loginCmd.Flags().String("token", "", "API token for authentication")
	loginCmd.MarkFlagRequired("token")

	logger.Debug("CLI commands and flags initialized")
}

func main() {
	defer logger.Close()

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed: %v", err)
		os.Exit(1)
	}
}