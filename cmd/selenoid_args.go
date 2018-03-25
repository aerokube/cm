package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
	"os"
)

var selenoidArgsCmd = &cobra.Command{
	Use:   "args",
	Short: "Shows Selenoid available args",
	Run: func(cmd *cobra.Command, args []string) {
		argsImpl(uiConfigDir, uiPort, func(lc *selenoid.Lifecycle) error {
			return lc.PrintArgs()
		}, force)
	},
}

func argsImpl(configDir string, port uint16, argsAction func(*selenoid.Lifecycle) error, force bool) {
	lifecycle, err := createLifecycle(configDir, port)
	if err != nil {
		stderr("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	lifecycle.Force = force
	err = argsAction(lifecycle)
	if err != nil {
		lifecycle.Errorf("Failed to print args: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}
