package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove Selenoid traces",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle()
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}

		err = lifecycle.Stop()
		if err != nil {
			stderr("Failed to stop Selenoid: %v\n", err)
			os.Exit(1)
		}

		err = os.RemoveAll(configDir)
		if err != nil {
			lifecycle.Printf("Failed to remove configuration directory: %v\n", err)
			os.Exit(1)
		}
		lifecycle.Printf("Successfully removed configuration directory\n")
		os.Exit(0)
	},
}
