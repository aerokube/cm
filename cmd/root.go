package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	verbose  bool
	registry string
	rootCmd  = &cobra.Command{
		Use:   "cm",
		Short: "cm is a configuration management tool for Aerokube products",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
)

const (
	registryUrl = "https://registry.hub.docker.com"
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&registry, "registry", "r", registryUrl, "Docker registry to use")
	rootCmd.AddCommand(selenoidCmd)
}

func Execute() {
	if _, err := rootCmd.ExecuteC(); err != nil {
		os.Exit(1)
	}
}
