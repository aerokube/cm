package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows Selenoid configuration status",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle(configDir, port)
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		lifecycle.Status()
	},
}
