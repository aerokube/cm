package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
	"os"
)

var selenoidStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Selenoid",
	Run: func(cmd *cobra.Command, args []string) {
		startImpl(configDir, func(lc *selenoid.Lifecycle) error {
			return lc.Start()
		}, force)
	},
}

func startImpl(configDir string, startAction func(*selenoid.Lifecycle) error, force bool) {
	lifecycle, err := createLifecycle(configDir)
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	lifecycle.Force = force
	err = startAction(lifecycle)
	if err != nil {
		lifecycle.Printf("Failed to start: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
