package main

import (
	"fmt"
	"os"

	"giraffecloud/internal/config"
	"giraffecloud/internal/service"
	"giraffecloud/internal/tunnel"

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

		t := tunnel.NewTunnel(cfg)
		if err := t.Connect(); err != nil {
			fmt.Printf("Error connecting to GiraffeCloud: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully connected to GiraffeCloud")
		// TODO: Implement signal handling for graceful shutdown
		select {}
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
		sm, err := service.NewServiceManager()
		if err != nil {
			fmt.Printf("Error creating service manager: %v\n", err)
			os.Exit(1)
		}

		if err := sm.Install(); err != nil {
			fmt.Printf("Error installing service: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully installed GiraffeCloud service")
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall GiraffeCloud system service",
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := service.NewServiceManager()
		if err != nil {
			fmt.Printf("Error creating service manager: %v\n", err)
			os.Exit(1)
		}

		if err := sm.Uninstall(); err != nil {
			fmt.Printf("Error uninstalling service: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully uninstalled GiraffeCloud service")
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