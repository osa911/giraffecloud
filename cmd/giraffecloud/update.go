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
			logger.Info("Latest Version: %s", versionInfo.LatestVersion)
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
		// Load existing config
		existingCfg, err := tunnel.LoadConfig()
		if err != nil && !os.IsNotExist(err) {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Create new config with auto-update enabled
		newCfg := &tunnel.Config{
			AutoUpdate: tunnel.AutoUpdateConfig{
				Enabled: true,
			},
		}

		// Merge changes
		mergedCfg := tunnel.MergeConfig(existingCfg, newCfg)

		// Save config
		if err := tunnel.SaveConfig(mergedCfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Automatic updates enabled")
		logger.Info("💡 Updates will be checked every %v", mergedCfg.AutoUpdate.CheckInterval)
		if mergedCfg.AutoUpdate.RequiredOnly {
			logger.Info("💡 Only required updates will be installed automatically")
		}
	},
}

var autoUpdateDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable automatic updates",
	Run: func(cmd *cobra.Command, args []string) {
		// Load existing config
		existingCfg, err := tunnel.LoadConfig()
		if err != nil && !os.IsNotExist(err) {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Create new config with auto-update disabled
		newCfg := &tunnel.Config{
			AutoUpdate: tunnel.AutoUpdateConfig{
				Enabled: false,
			},
		}

		// Merge changes
		mergedCfg := tunnel.MergeConfig(existingCfg, newCfg)

		// Save config
		if err := tunnel.SaveConfig(mergedCfg); err != nil {
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
		// Load existing config
		existingCfg, err := tunnel.LoadConfig()
		if err != nil && !os.IsNotExist(err) {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Create new config with auto-update settings
		newCfg := &tunnel.Config{
			AutoUpdate: tunnel.AutoUpdateConfig{},
		}

		// Update settings from flags
		if interval, _ := cmd.Flags().GetString("interval"); interval != "" {
			if duration, err := time.ParseDuration(interval); err == nil {
				newCfg.AutoUpdate.CheckInterval = duration
				logger.Info("✅ Check interval set to: %v", duration)
			} else {
				logger.Error("Invalid interval format: %s", interval)
				os.Exit(1)
			}
		}

		if requiredOnly, _ := cmd.Flags().GetBool("required-only"); cmd.Flags().Changed("required-only") {
			newCfg.AutoUpdate.RequiredOnly = requiredOnly
			logger.Info("✅ Required-only mode: %v", requiredOnly)
		}

		if preserveConn, _ := cmd.Flags().GetBool("preserve-connection"); cmd.Flags().Changed("preserve-connection") {
			newCfg.AutoUpdate.PreserveConnection = preserveConn
			logger.Info("✅ Preserve connection: %v", preserveConn)
		}

		if restartService, _ := cmd.Flags().GetBool("restart-service"); cmd.Flags().Changed("restart-service") {
			newCfg.AutoUpdate.RestartService = restartService
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
							newCfg.AutoUpdate.UpdateWindow = &tunnel.TimeWindow{
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

		// Merge changes
		mergedCfg := tunnel.MergeConfig(existingCfg, newCfg)

		// Save config
		if err := tunnel.SaveConfig(mergedCfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			os.Exit(1)
		}

		logger.Info("✅ Auto-update configuration saved successfully")
	},
}

// initUpdateCommands sets up all update-related commands and their flags
func initUpdateCommands() {
	// Add update commands to root
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(autoUpdateCmd)

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
}