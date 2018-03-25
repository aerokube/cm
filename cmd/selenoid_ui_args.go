package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
)

var selenoidUIArgsCmd = &cobra.Command{
	Use:   "args",
	Short: "Shows Selenoid UI available args",
	Run: func(cmd *cobra.Command, args []string) {
		argsImpl(uiConfigDir, uiPort, func(lc *selenoid.Lifecycle) error {
			return lc.PrintUIArgs()
		}, force)
	},
}
