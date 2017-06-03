package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Selenoid",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle()
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		err = lifecycle.Stop()
		if err != nil {
			lifecycle.Printf("Failed to stop Selenoid: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}
