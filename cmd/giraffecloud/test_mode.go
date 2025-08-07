package main

import (
	"giraffecloud/internal/tunnel"
	"os"

	"github.com/spf13/cobra"
)

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
		// Load existing config
		existingCfg, err := tunnel.LoadConfig()
		if err != nil && !os.IsNotExist(err) {
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

		// Create new config with just the test mode changes
		newCfg := &tunnel.Config{
			TestMode: tunnel.TestModeConfig{
				Enabled: true,
				Channel: channel,
			},
			AutoUpdate: tunnel.AutoUpdateConfig{
				Channel: channel,
			},
		}

		// Merge changes
		mergedCfg := tunnel.MergeConfig(existingCfg, newCfg)

		// Save config
		if err := tunnel.SaveConfig(mergedCfg); err != nil {
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
		// Load existing config
		existingCfg, err := tunnel.LoadConfig()
		if err != nil && !os.IsNotExist(err) {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Create new config with test mode disabled
		newCfg := &tunnel.Config{
			TestMode: tunnel.TestModeConfig{
				Enabled: false,
				Channel: "stable",
			},
			AutoUpdate: tunnel.AutoUpdateConfig{
				Channel: "stable",
			},
		}

		// Merge changes
		mergedCfg := tunnel.MergeConfig(existingCfg, newCfg)

		// Save config
		if err := tunnel.SaveConfig(mergedCfg); err != nil {
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
	},
}

// initTestModeCommands sets up all test mode commands
func initTestModeCommands() {
	// Add test mode command to root
	rootCmd.AddCommand(testModeCmd)

	// Add subcommands to test mode
	testModeCmd.AddCommand(testModeEnableCmd)
	testModeCmd.AddCommand(testModeDisableCmd)
	testModeCmd.AddCommand(testModeStatusCmd)
}