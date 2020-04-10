package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version string

func init() {
	rootCmd.AddCommand(versionCommand)
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print dgit version",
	Long:  "Output dgit version to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dgit version %s\n", Version)
	},
}
