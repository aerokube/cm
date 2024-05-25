package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var selenoidUIStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows Selenoid UI status",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle(uiConfigDir, uiPort)
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		lifecycle.UIStatus()
	},
}
