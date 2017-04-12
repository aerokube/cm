package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	verbose bool
	rootCmd = &cobra.Command{
		Use:   "cm",
		Short: "cm is a configuration management tool for Aerokube products",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.AddCommand(selenoidCmd)
}

func Execute() {
	if _, err := rootCmd.ExecuteC(); err != nil {
		os.Exit(1)
	}
}
