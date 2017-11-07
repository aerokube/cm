package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
)

var selenoidCleanupUICmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove Selenoid UI traces",
	Run: func(cmd *cobra.Command, args []string) {
		cleanupImpl(uiConfigDir, uiPort, func(lc *selenoid.Lifecycle) error {
			return lc.StopUI()
		})
	},
}
