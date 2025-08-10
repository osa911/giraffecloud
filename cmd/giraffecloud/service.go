package main

import (
	"fmt"
	"giraffecloud/internal/tunnel"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// serviceCmd handles system service management
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

var healthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Check GiraffeCloud system service health and status",
	Long: `Check the status and health of the installed GiraffeCloud system service including:
- Service installation status
- Service running status
- Service logs and recent activity
- Configuration validity
- Certificate status

This helps diagnose issues with the system service.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("=== GiraffeCloud System Service Health Check ===")

		// Create service manager
		sm, err := tunnel.NewServiceManager()
		if err != nil {
			logger.Error("❌ Service Manager: Failed to create service manager: %v", err)
			os.Exit(1)
		}
		logger.Info("✅ Service Manager: Initialized")

		// Check if service is installed
		isInstalled, err := sm.IsInstalled()
		if err != nil {
			logger.Error("❌ Installation Check: Failed to check service installation: %v", err)
		} else if !isInstalled {
			logger.Error("❌ Installation: GiraffeCloud service is not installed")
			logger.Info("💡 Tip: Run 'giraffecloud service install' to install the service")
			os.Exit(1)
		} else {
			logger.Info("✅ Installation: GiraffeCloud service is installed")
		}

		// Check if service is running
		isRunning, err := sm.IsRunning()
		if err != nil {
			logger.Error("❌ Service Status: Failed to check service status: %v", err)
		} else if !isRunning {
			logger.Error("❌ Service Status: GiraffeCloud service is not running")
			logger.Info("💡 Tip: Run 'giraffecloud service start' to start the service")
		} else {
			logger.Info("✅ Service Status: GiraffeCloud service is running")
		}

		// Load configuration to check validity
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("❌ Configuration: Failed to load config file: %v", err)
			logger.Info("💡 Tip: Run 'giraffecloud login --token YOUR_TOKEN' to create configuration")
		} else {
			logger.Info("✅ Configuration: Config file loaded successfully")

			// Check if we have a token
			if cfg.Token == "" {
				logger.Error("❌ Authentication: No API token found in configuration")
				logger.Info("💡 Tip: Run 'giraffecloud login --token YOUR_TOKEN' to authenticate")
			} else {
				logger.Info("✅ Authentication: API token present")
			}

			// Check certificates
			certificatesOK := true
			if cfg.Security.CACert != "" {
				if _, err := os.Stat(cfg.Security.CACert); os.IsNotExist(err) {
					logger.Error("❌ Certificates: CA certificate not found at %s", cfg.Security.CACert)
					certificatesOK = false
				} else {
					logger.Info("✅ Certificates: CA certificate found")
				}
			}

			if cfg.Security.ClientCert != "" {
				if _, err := os.Stat(cfg.Security.ClientCert); os.IsNotExist(err) {
					logger.Error("❌ Certificates: Client certificate not found at %s", cfg.Security.ClientCert)
					certificatesOK = false
				} else {
					logger.Info("✅ Certificates: Client certificate found")
				}
			}

			if cfg.Security.ClientKey != "" {
				if _, err := os.Stat(cfg.Security.ClientKey); os.IsNotExist(err) {
					logger.Error("❌ Certificates: Client key not found at %s", cfg.Security.ClientKey)
					certificatesOK = false
				} else {
					logger.Info("✅ Certificates: Client key found")
				}
			}

			if !certificatesOK {
				logger.Info("💡 Tip: Run 'giraffecloud login --token YOUR_TOKEN' to download certificates")
			}

			// Check tunnel server connectivity
			logger.Info("🔍 Testing tunnel server connectivity...")
			serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

			conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
			if err != nil {
				logger.Error("❌ Tunnel Server: Cannot connect to %s - %v", serverAddr, err)
				logger.Info("💡 Tip: Check your internet connection and firewall settings")
			} else {
				conn.Close()
				logger.Info("✅ Tunnel Server: %s is reachable", serverAddr)
			}
		}

		// Try to get service logs if available
		showLogs, _ := cmd.Flags().GetBool("show-logs")
		if showLogs {
			logger.Info("🔍 Fetching recent service logs...")
			logs, err := sm.GetLogs()
			if err != nil {
				logger.Error("❌ Service Logs: Failed to get service logs: %v", err)
			} else if logs == "" {
				logger.Info("ℹ️  Service Logs: No recent logs available")
			} else {
				logger.Info("📋 Recent Service Logs:")
				logger.Info("%s", logs)
			}
		}

		// Summary
		logger.Info("")
		logger.Info("=== Health Check Summary ===")
		if cfg != nil {
			logger.Info("Domain: %s", cfg.Domain)
			logger.Info("Local Port: %d", cfg.LocalPort)
			logger.Info("Tunnel Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
		}

		if isInstalled && isRunning {
			logger.Info("🎉 System service is installed and running!")
			if cfg != nil && cfg.Token != "" {
				logger.Info("💡 Your tunnel should be active at: https://%s", cfg.Domain)
			}
		} else {
			logger.Info("⚠️  System service issues found. Please address them.")
			if !isInstalled {
				logger.Info("💡 Next step: giraffecloud service install")
			} else if !isRunning {
				logger.Info("💡 Next step: giraffecloud service start")
			}
		}
	},
}

// initServiceCommands sets up all service-related commands and their flags
func initServiceCommands() {
	// Add subcommands to service
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	serviceCmd.AddCommand(healthCheckCmd)

	// Start service
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start GiraffeCloud service",
		Run: func(cmd *cobra.Command, args []string) {
			sm, err := tunnel.NewServiceManager()
			if err != nil {
				logger.Error("Failed to create service manager: %v", err)
				os.Exit(1)
			}
			if err := sm.Start(); err != nil {
				logger.Error("Failed to start service: %v", err)
				logger.Info("Tip: You may need elevated privileges (try with sudo)")
				os.Exit(1)
			}
			logger.Info("Service started")
		},
	}
	serviceCmd.AddCommand(startCmd)

	// Stop service
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop GiraffeCloud service",
		Run: func(cmd *cobra.Command, args []string) {
			sm, err := tunnel.NewServiceManager()
			if err != nil {
				logger.Error("Failed to create service manager: %v", err)
				os.Exit(1)
			}
			if err := sm.Stop(); err != nil {
				logger.Error("Failed to stop service: %v", err)
				logger.Info("Tip: You may need elevated privileges (try with sudo)")
				os.Exit(1)
			}
			logger.Info("Service stopped")
		},
	}
	serviceCmd.AddCommand(stopCmd)

	// Restart service
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart GiraffeCloud service",
		Run: func(cmd *cobra.Command, args []string) {
			sm, err := tunnel.NewServiceManager()
			if err != nil {
				logger.Error("Failed to create service manager: %v", err)
				os.Exit(1)
			}
			if err := sm.Restart(); err != nil {
				logger.Error("Failed to restart service: %v", err)
				logger.Info("Tip: You may need elevated privileges (try with sudo)")
				os.Exit(1)
			}
		},
	}
	serviceCmd.AddCommand(restartCmd)

	// Status service
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show service status",
		Run: func(cmd *cobra.Command, args []string) {
			sm, err := tunnel.NewServiceManager()
			if err != nil {
				logger.Error("Failed to create service manager: %v", err)
				os.Exit(1)
			}
			installed, ierr := sm.IsInstalled()
			running, rerr := sm.IsRunning()
			if ierr != nil {
				logger.Error("Failed to check installation: %v", ierr)
			}
			if rerr != nil {
				logger.Error("Failed to check running state: %v", rerr)
			}
			logger.Info("Installed: %v", installed)
			logger.Info("Running: %v", running)
			if !installed {
				logger.Info("Tip: Run 'giraffecloud service install'")
			} else if !running {
				logger.Info("Tip: Run 'giraffecloud service start'")
			}
		},
	}
	serviceCmd.AddCommand(statusCmd)

	// Logs
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show service logs",
		Run: func(cmd *cobra.Command, args []string) {
			follow, _ := cmd.Flags().GetBool("follow")
			lines, _ := cmd.Flags().GetInt("lines")
			sm, err := tunnel.NewServiceManager()
			if err != nil {
				logger.Error("Failed to create service manager: %v", err)
				os.Exit(1)
			}
			if follow {
				if err := sm.FollowLogsWithLines(lines); err != nil {
					logger.Error("Failed to follow logs: %v", err)
					os.Exit(1)
				}
				return
			}
			logs, err := sm.GetLogsWithLines(lines)
			if err != nil {
				logger.Error("Failed to get service logs: %v", err)
				os.Exit(1)
			}
			if logs == "" {
				logger.Info("No recent logs available")
				return
			}
			logger.Info("%s", logs)
		},
	}
	serviceCmd.AddCommand(logsCmd)

	// Add flags to health-check command
	// System-level only; user-level flags removed
	logsCmd.Flags().Bool("follow", false, "Follow live logs (Linux/macOS)")
	logsCmd.Flags().Int("lines", 200, "Number of recent log lines to show/start from")
	healthCheckCmd.Flags().Bool("show-logs", false, "Show recent service logs")
}
