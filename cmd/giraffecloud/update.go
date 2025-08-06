package main

import (
	"fmt"
	"giraffecloud/internal/service"
	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

// updateCmd handles manual client updates
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update GiraffeCloud CLI client to the latest version",
	Long: `Check for and install updates to the GiraffeCloud CLI client.
This will download the latest version from GitHub releases and replace the current executable.

Examples:
  giraffecloud update                    # Check and install updates
  giraffecloud update --check-only       # Only check for updates, don't install
  giraffecloud update --force            # Force update even if same version`,
	Run: func(cmd *cobra.Command, args []string) {
		checkOnly, _ := cmd.Flags().GetBool("check-only")
		force, _ := cmd.Flags().GetBool("force")

		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Create updater service
		downloadURL := "https://github.com/osa911/giraffecloud/releases/download"
		updater, err := service.NewUpdaterService(downloadURL)
		if err != nil {
			logger.Error("Failed to create updater service: %v", err)
			os.Exit(1)
		}

		// Check for updates
		logger.Info("Checking for updates...")
		serverURL := fmt.Sprintf("https://%s:%d", cfg.API.Host, cfg.API.Port)
		updateInfo, err := updater.CheckForUpdates(serverURL)
		if err != nil {
			logger.Error("Failed to check for updates: %v", err)
			os.Exit(1)
		}

		if updateInfo == nil {
			logger.Info("✅ You are already running the latest version: %s", version.Version)
			return
		}

		logger.Info("🆕 Update available!")
		logger.Info("   Current version: %s", updateInfo.CurrentVersion)
		logger.Info("   Latest version:  %s", updateInfo.Version)
		if updateInfo.IsRequired {
			logger.Info("   ⚠️  This update is REQUIRED")
		}

		if checkOnly {
			logger.Info("Use 'giraffecloud update' to install the update")
			return
		}

		// Ask for confirmation unless forced or required
		if !force && !updateInfo.IsRequired {
			logger.Info("Do you want to install this update? (y/N)")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				logger.Info("Update cancelled")
				return
			}
		}

		// Download update
		logger.Info("📥 Downloading update...")
		s := spinner.New(spinner.CharSets[14], 120*time.Millisecond)
		s.Suffix = " Downloading update..."
		s.Start()

		downloadPath, err := updater.DownloadUpdate(updateInfo)
		s.Stop()

		if err != nil {
			logger.Error("Failed to download update: %v", err)
			os.Exit(1)
		}

		// Install update
		logger.Info("🔧 Installing update...")
		s.Suffix = " Installing update..."
		s.Start()

		err = updater.InstallUpdate(downloadPath)
		s.Stop()

		if err != nil {
			logger.Error("Failed to install update: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Update completed successfully!")
		logger.Info("🎉 GiraffeCloud has been updated to version %s", updateInfo.Version)
		logger.Info("💡 You may need to restart any running services")

		// Clean up old backups
		if err := updater.CleanupOldBackups(); err != nil {
			logger.Warn("Failed to cleanup old backups: %v", err)
		}
	},
}

// autoUpdateCmd handles automatic update configuration
var autoUpdateCmd = &cobra.Command{
	Use:   "auto-update",
	Short: "Manage automatic updates",
	Long:  `Configure and manage automatic updates for the GiraffeCloud CLI client.`,
}

var autoUpdateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show auto-update service status",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		logger.Info("=== Auto-Update Configuration ===")
		logger.Info("Enabled: %v", cfg.AutoUpdate.Enabled)
		logger.Info("Check Interval: %v", cfg.AutoUpdate.CheckInterval)
		logger.Info("Required Only: %v", cfg.AutoUpdate.RequiredOnly)
		logger.Info("Download URL: %s", cfg.AutoUpdate.DownloadURL)
		logger.Info("Preserve Connection: %v", cfg.AutoUpdate.PreserveConnection)
		logger.Info("Restart Service: %v", cfg.AutoUpdate.RestartService)
		logger.Info("Backup Count: %d", cfg.AutoUpdate.BackupCount)

		if cfg.AutoUpdate.UpdateWindow != nil {
			logger.Info("Update Window: %02d:00-%02d:00 %s",
				cfg.AutoUpdate.UpdateWindow.StartHour,
				cfg.AutoUpdate.UpdateWindow.EndHour,
				cfg.AutoUpdate.UpdateWindow.Timezone)
		} else {
			logger.Info("Update Window: Any time")
		}

		// Check current update status
		logger.Info("\n=== Update Status ===")
		serverURL := fmt.Sprintf("https://%s:%d", cfg.API.Host, cfg.API.Port)
		versionInfo, err := version.CheckServerVersion(serverURL)
		if err != nil {
			logger.Error("Failed to check server version: %v", err)
		} else {
			logger.Info("Current Version: %s", version.Version)
			logger.Info("Server Version: %s", versionInfo.ServerVersion)
			if versionInfo.UpdateAvailable {
				logger.Info("Update Available: ✅ YES")
				if versionInfo.UpdateRequired {
					logger.Info("Update Required: ⚠️  YES")
				} else {
					logger.Info("Update Required: No")
				}
			} else {
				logger.Info("Update Available: No")
			}
		}
	},
}

var autoUpdateEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable automatic updates",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		cfg.AutoUpdate.Enabled = true

		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Automatic updates enabled")
		logger.Info("💡 Updates will be checked every %v", cfg.AutoUpdate.CheckInterval)
		if cfg.AutoUpdate.RequiredOnly {
			logger.Info("💡 Only required updates will be installed automatically")
		}
	},
}

var autoUpdateDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable automatic updates",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		cfg.AutoUpdate.Enabled = false

		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Automatic updates disabled")
		logger.Info("💡 You can still manually update using 'giraffecloud update'")
	},
}

var autoUpdateConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure auto-update settings",
	Long: `Configure automatic update settings.

Examples:
  giraffecloud auto-update config --interval=12h    # Check every 12 hours
  giraffecloud auto-update config --required-only   # Only install required updates
  giraffecloud auto-update config --window=2,6,UTC  # Update between 2-6 AM UTC`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Update settings from flags
		if interval, _ := cmd.Flags().GetString("interval"); interval != "" {
			if duration, err := time.ParseDuration(interval); err == nil {
				cfg.AutoUpdate.CheckInterval = duration
				logger.Info("✅ Check interval set to: %v", duration)
			} else {
				logger.Error("Invalid interval format: %s", interval)
				os.Exit(1)
			}
		}

		if requiredOnly, _ := cmd.Flags().GetBool("required-only"); cmd.Flags().Changed("required-only") {
			cfg.AutoUpdate.RequiredOnly = requiredOnly
			logger.Info("✅ Required-only mode: %v", requiredOnly)
		}

		if preserveConn, _ := cmd.Flags().GetBool("preserve-connection"); cmd.Flags().Changed("preserve-connection") {
			cfg.AutoUpdate.PreserveConnection = preserveConn
			logger.Info("✅ Preserve connection: %v", preserveConn)
		}

		if restartService, _ := cmd.Flags().GetBool("restart-service"); cmd.Flags().Changed("restart-service") {
			cfg.AutoUpdate.RestartService = restartService
			logger.Info("✅ Restart service: %v", restartService)
		}

		if window, _ := cmd.Flags().GetString("window"); window != "" {
			parts := strings.Split(window, ",")
			if len(parts) == 3 {
				// Parse start hour, end hour, timezone
				var startHour, endHour int
				var timezone string
				if _, err1 := fmt.Sscanf(parts[0], "%d", &startHour); err1 == nil {
					if _, err2 := fmt.Sscanf(parts[1], "%d", &endHour); err2 == nil {
						if startHour >= 0 && startHour <= 23 && endHour >= 0 && endHour <= 23 {
							timezone = strings.TrimSpace(parts[2])
							cfg.AutoUpdate.UpdateWindow = &tunnel.TimeWindow{
								StartHour: startHour,
								EndHour:   endHour,
								Timezone:  timezone,
							}
							logger.Info("✅ Update window set to: %02d:00-%02d:00 %s", startHour, endHour, timezone)
						} else {
							logger.Error("Invalid window format. Hours must be 0-23")
							os.Exit(1)
						}
					} else {
						logger.Error("Invalid end hour format")
						os.Exit(1)
					}
				} else {
					logger.Error("Invalid start hour format")
					os.Exit(1)
				}
			} else {
				logger.Error("Invalid window format. Use: start_hour,end_hour,timezone (e.g., 2,6,UTC)")
				os.Exit(1)
			}
		}

		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Auto-update configuration saved successfully")
	},
}

// testModeCmd handles test mode configuration for pre-release testing
var testModeCmd = &cobra.Command{
	Use:   "test-mode",
	Short: "Manage test mode settings",
	Long:  `Configure and manage test mode settings for receiving pre-release updates.`,
}

var testModeEnableCmd = &cobra.Command{
	Use:   "enable [channel]",
	Short: "Enable test mode with optional channel",
	Long: `Enable test mode to receive pre-release updates.
Channels: stable, beta, test

Examples:
  giraffecloud test-mode enable         # Enable with stable channel
  giraffecloud test-mode enable beta    # Enable with beta channel
  giraffecloud test-mode enable test    # Enable with test channel`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Determine channel
		channel := "stable"
		if len(args) > 0 {
			switch args[0] {
			case "stable", "beta", "test":
				channel = args[0]
			default:
				logger.Error("Invalid channel: %s. Valid channels: stable, beta, test", args[0])
				os.Exit(1)
			}
		}

		// Update config
		cfg.TestMode.Enabled = true
		cfg.TestMode.Channel = channel
		cfg.AutoUpdate.Channel = channel

		// Save config
		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Test mode enabled with channel: %s", channel)
		logger.Info("💡 Use 'giraffecloud update --check-only' to check for updates")
		logger.Info("💡 Use 'giraffecloud test-mode disable' to disable test mode")
	},
}

var testModeDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable test mode",
	Long:  `Disable test mode and return to stable release channel.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Update config
		cfg.TestMode.Enabled = false
		cfg.TestMode.Channel = "stable"
		cfg.AutoUpdate.Channel = "stable"

		// Save config
		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Test mode disabled")
		logger.Info("💡 Switched back to stable release channel")
	},
}

var testModeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show test mode status",
	Long:  `Display current test mode configuration and status.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		logger.Info("=== Test Mode Status ===")
		if cfg.TestMode.Enabled {
			logger.Info("Status: ✅ ENABLED")
			logger.Info("Channel: %s", cfg.TestMode.Channel)
			if cfg.TestMode.UserID != "" {
				logger.Info("User ID: %s", cfg.TestMode.UserID)
			}
			if len(cfg.TestMode.Groups) > 0 {
				logger.Info("Test Groups: %v", cfg.TestMode.Groups)
			}
		} else {
			logger.Info("Status: ❌ DISABLED")
			logger.Info("Channel: stable (default)")
		}

		logger.Info("")
		logger.Info("Auto-Update Channel: %s", cfg.AutoUpdate.Channel)
		logger.Info("Auto-Update Enabled: %t", cfg.AutoUpdate.Enabled)

		// Check for updates
		if cfg.API.Host != "" {
			logger.Info("")
			logger.Info("Checking for updates...")
			serverURL := fmt.Sprintf("https://%s:%d", cfg.API.Host, cfg.API.Port)

			// Add channel parameter for version check
			versionURL := serverURL + "/api/v1/tunnels/version"
			versionURL += "?client_version=" + version.Version
			if cfg.TestMode.Enabled {
				versionURL += "&channel=" + cfg.TestMode.Channel
			}

			versionInfo, err := version.CheckServerVersion(serverURL)
			if err != nil {
				logger.Warn("Could not check server version: %v", err)
			} else {
				if versionInfo.UpdateAvailable {
					logger.Info("📢 Update available: %s -> %s", version.Version, versionInfo.ServerVersion)
					if versionInfo.UpdateRequired {
						logger.Info("⚠️  This update is REQUIRED")
					}
				} else {
					logger.Info("✅ You are running the latest version")
				}
			}
		}
	},
}

var testModeSetUserCmd = &cobra.Command{
	Use:   "set-user [user-id]",
	Short: "Set user ID for test targeting",
	Long:  `Set a user ID for test targeting. This allows server-side configuration of which users receive specific test versions.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		cfg.TestMode.UserID = args[0]

		if err := tunnel.SaveConfig(cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Test user ID set to: %s", args[0])
	},
}

// initUpdateCommands sets up all update-related commands and their flags
func initUpdateCommands() {
	// Add flags to update command
	updateCmd.Flags().Bool("check-only", false, "Only check for updates, don't install")
	updateCmd.Flags().Bool("force", false, "Force update even if same version")

	// Add subcommands to auto-update
	autoUpdateCmd.AddCommand(autoUpdateStatusCmd)
	autoUpdateCmd.AddCommand(autoUpdateEnableCmd)
	autoUpdateCmd.AddCommand(autoUpdateDisableCmd)
	autoUpdateCmd.AddCommand(autoUpdateConfigCmd)

	// Add flags to auto-update config command
	autoUpdateConfigCmd.Flags().String("interval", "", "Check for updates every duration (e.g., 1h, 12h, 24h)")
	autoUpdateConfigCmd.Flags().Bool("required-only", false, "Only install required updates")
	autoUpdateConfigCmd.Flags().Bool("preserve-connection", false, "Preserve existing tunnel connection during update")
	autoUpdateConfigCmd.Flags().Bool("restart-service", false, "Restart the GiraffeCloud service after update")
	autoUpdateConfigCmd.Flags().String("window", "", "Update window in HH:MM-HH:MM format (e.g., 2,6,UTC)")

	// Add subcommands to test-mode
	testModeCmd.AddCommand(testModeEnableCmd)
	testModeCmd.AddCommand(testModeDisableCmd)
	testModeCmd.AddCommand(testModeStatusCmd)
	testModeCmd.AddCommand(testModeSetUserCmd)
}