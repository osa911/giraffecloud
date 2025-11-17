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
	Short: "Show configuration file paths",
	Long:  `Display paths to existing GiraffeCloud configuration files and directories.`,
	Run: func(cmd *cobra.Command, args []string) {
		baseDir, err := tunnel.GetConfigDir()
		if err != nil {
			logger.Error("Failed to determine config directory: %v", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration directory: %s\n", baseDir)

		// Only show files/directories that actually exist
		configPath := filepath.Join(baseDir, "config.json")
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("Config file:            %s\n", configPath)
		}

		certsDir := filepath.Join(baseDir, "certs")
		if _, err := os.Stat(certsDir); err == nil {
			fmt.Printf("Certificates directory: %s\n", certsDir)
			// List certificate files if they exist
			certFiles := []string{"ca.crt", "client.crt", "client.key"}
			for _, certFile := range certFiles {
				certPath := filepath.Join(certsDir, certFile)
				if _, err := os.Stat(certPath); err == nil {
					fmt.Printf("  - %s\n", certPath)
				}
			}
		}

		logPath := filepath.Join(baseDir, "client.log")
		if _, err := os.Stat(logPath); err == nil {
			fmt.Printf("Log file:               %s\n", logPath)
		}

		// If nothing exists, show helpful message
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if _, err := os.Stat(certsDir); os.IsNotExist(err) {
				fmt.Printf("\nNo configuration found. Run 'giraffecloud login --token YOUR_API_TOKEN' to set up.\n")
			}
		}
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
