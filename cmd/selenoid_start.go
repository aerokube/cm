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
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	lifecycle.Force = force
	err = lifecycle.Start()
	if err != nil {
		lifecycle.Printf("Failed to start Selenoid: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
