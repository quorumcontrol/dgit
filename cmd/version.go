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
	Short: "Print decentragit version",
	Long:  "Output decentragit version to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("decentragit version %s\n", Version)
	},
}
