package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	quiet    bool
	registry string
	rootCmd  = &cobra.Command{
		Use:   "cm",
		Short: "cm is a configuration management tool for Aerokube products",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
)

func init() {
	rootCmd.AddCommand(selenoidCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if _, err := rootCmd.ExecuteC(); err != nil {
		os.Exit(1)
	}
}
