package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var selenoidDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Selenoid latest or specified release",
	Run: func(cmd *cobra.Command, args []string) {
		lifecycle, err := createLifecycle()
		if err != nil {
			stderr("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		err = lifecycle.Download()
		if err != nil {
			lifecycle.Printf("Failed to download Selenoid release: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}
