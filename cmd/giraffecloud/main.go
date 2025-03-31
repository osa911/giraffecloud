package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"giraffecloud/internal/config"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/service"
	"giraffecloud/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	logger *logging.Logger
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
			logger.Error("Failed to load config: %v", err)
			os.Exit(1)
		}

		// Initialize logger
		logger, err = logging.NewLogger(&cfg.Logging)
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		defer logger.Close()

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

		// Initialize logger
		logger, err = logging.NewLogger(&cfg.Logging)
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		defer logger.Close()

		logger.Info("Installing GiraffeCloud service")

		sm, err := service.NewServiceManager()
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

		// Initialize logger
		logger, err = logging.NewLogger(&cfg.Logging)
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		defer logger.Close()

		logger.Info("Uninstalling GiraffeCloud service")

		sm, err := service.NewServiceManager()
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

func init() {
	rootCmd.AddCommand(connectCmd)
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(serviceCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}