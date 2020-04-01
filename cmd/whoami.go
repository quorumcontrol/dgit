package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(whoAmICommand)
}

var whoAmICommand = &cobra.Command{
	Use:   "whoami",
	Short: "Print out your username in the current repo",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		callingDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting current workdir: %w", err)
			os.Exit(1)
		}

		repo := openRepo(cmd, callingDir)

		username, err := repo.Username()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Println(username)
	},
}
