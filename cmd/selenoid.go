package cmd

import (
	"fmt"
	"github.com/aerokube/cm/selenoid"
	"github.com/spf13/cobra"
	"os"
)

var (
	limit int
	pull  bool
	tmpfs int
)

func init() {
	selenoidCmd.Flags().IntVarP(&limit, "limit", "l", 5, "process only last N versions")
	selenoidCmd.Flags().BoolVarP(&pull, "pull", "p", false, "pull images if not present")
	selenoidCmd.Flags().IntVarP(&tmpfs, "tmpfs", "t", 512, "add tmpfs volume sized in megabytes")
}

var selenoidCmd = &cobra.Command{
	Use:   "selenoid",
	Short: "Generate JSON configuration for Selenoid",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := selenoid.Configurator{Limit: limit, Verbose: verbose, Pull: pull, RegistryUrl: registry, Tmpfs: tmpfs}
		err := cfg.Init()
		defer cfg.Close()
		if err != nil {
			fmt.Printf("Failed to initialize: %v\n", err)
			os.Exit(1)
		}
		data, err := cfg.Configure()
		if err != nil {
			fmt.Printf("Failed to configure: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(data)
		os.Exit(0)
	},
}
