package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
	"os"
)

var selenoidCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove Selenoid traces",
	Run: func(cmd *cobra.Command, args []string) {
		cleanupImpl(configDir, func(lc *selenoid.Lifecycle) error {
			return lc.Stop()
		})
	},
}

func cleanupImpl(configDir string, stopAction func(*selenoid.Lifecycle) error) {
	lifecycle, err := createLifecycle(configDir)
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	err = stopAction(lifecycle)
	if err != nil {
		stderr("Failed to stop: %v\n", err)
		os.Exit(1)
	}

	err = os.RemoveAll(configDir)
	if err != nil {
		lifecycle.Printf("Failed to remove configuration directory: %v\n", err)
		os.Exit(1)
	}
	lifecycle.Printf("Successfully removed configuration directory\n")
	os.Exit(0)
}
