package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
)

var selenoidUpdateUICmd = &cobra.Command{
	Use:   "update",
	Short: "Update Selenoid UI (download latest Selenoid UI and start)",
	Run: func(cmd *cobra.Command, args []string) {
		startImpl(func(lc *selenoid.Lifecycle) error {
			return lc.StartUI()
		}, true)
	},
}
