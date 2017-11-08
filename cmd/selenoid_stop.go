package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
	"os"
)

var selenoidStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Selenoid",
	Run: func(cmd *cobra.Command, args []string) {
		stopImpl(configDir, port, func(lc *selenoid.Lifecycle) error {
			return lc.Stop()
		})
	},
}

func stopImpl(configDir string, port uint16, stopAction func(*selenoid.Lifecycle) error) {
	lifecycle, err := createLifecycle(configDir, port)
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	err = stopAction(lifecycle)
	if err != nil {
		lifecycle.Printf("Failed to stop: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
