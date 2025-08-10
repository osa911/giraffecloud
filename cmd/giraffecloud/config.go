package main

import (
	"encoding/json"
	"fmt"
	"giraffecloud/internal/tunnel"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GiraffeCloud configuration",
	Long:  `View and manage GiraffeCloud client configuration.`,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Long:  `Display the path to the GiraffeCloud configuration file.`,
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory: %v", err)
			os.Exit(1)
		}

		baseDir := filepath.Join(homeDir, ".giraffecloud")
		configPath := filepath.Join(baseDir, "config.json")
		certsDir := filepath.Join(baseDir, "certs")
		logPath := filepath.Join(baseDir, "client.log")

		fmt.Printf("Configuration directory: %s\n", baseDir)
		fmt.Printf("Config file:            %s\n", configPath)
		fmt.Printf("Certificates directory: %s\n", certsDir)
		fmt.Printf("Log file:              %s\n", logPath)
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current GiraffeCloud configuration in JSON format.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := tunnel.LoadConfig()
		if err != nil {
			logger.Error("Error loading config: %v", err)
			os.Exit(1)
		}

		// Show actual path being read
		path, _ := tunnel.GetConfigPath()
		fmt.Printf("Config file: %s\n", path)

		// Convert config to JSON with indentation
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal config: %v", err)
			os.Exit(1)
		}

		fmt.Println(string(data))
	},
}

// initConfigCommands sets up all config-related commands
func initConfigCommands() {
	// Add subcommands to config
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configShowCmd)
}
