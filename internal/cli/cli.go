package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "giraffecloud",
	Short: "GiraffeCloud CLI",
	Long:  `GiraffeCloud CLI helps you connect, manage, and expose tunnels with ease.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🦒 GiraffeCloud CLI is alive!")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}