package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
)

var selenoidStopUICmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Selenoid UI",
	Run: func(cmd *cobra.Command, args []string) {
		stopImpl(func(lc *selenoid.Lifecycle) error {
			return lc.StopUI()
		})
	},
}
