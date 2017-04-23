package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Selenoid",
	Run: func(cmd *cobra.Command, args []string) {
		startImpl(force)
	},
}

func startImpl(force bool) {
	lifecycle, err := createLifecycle()
	lifecycle.Force = force
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	err = lifecycle.Start()
	if err != nil {
		lifecycle.Printf("Failed to start Selenoid: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
