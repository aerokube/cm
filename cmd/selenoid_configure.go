package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Create Selenoid configuration file and download dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle(configDir)
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		err = lifecycle.Configure()
		if err != nil {
			lifecycle.Printf("Failed to configure Selenoid: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}
