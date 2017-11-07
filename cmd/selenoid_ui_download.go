package cmd

import (
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
)

var selenoidDownloadUICmd = &cobra.Command{
	Use:   "download",
	Short: "Download latest or specified release of Selenoid UI",
	Run: func(cmd *cobra.Command, args []string) {
		downloadImpl(uiConfigDir, uiPort, func(lc *selenoid.Lifecycle) error {
			return lc.DownloadUI()
		})
	},
}
