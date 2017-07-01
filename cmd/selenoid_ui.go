package cmd

import "github.com/spf13/cobra"

var selenoidUICmd = &cobra.Command{
	Use:   "selenoid-ui",
	Short: "Download, configure and run Selenoid UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}
