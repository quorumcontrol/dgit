package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/quorumcontrol/dgit/msg"
)

func init() {
	rootCmd.AddCommand(teamCommand)
}

var teamCommand = &cobra.Command{
	Use:   "team (add [usernames] | list | remove [usernames])",
	Short: "Manage your repo's team of collaborators",
	Args: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "add", "remove":
			if len(args) < 2 {
				return fmt.Errorf("%s command requires one or more usernames to %s", args[0], args[0])
			}
			return nil
		case "list":
			if len(args) != 1 {
				return fmt.Errorf("unexpected arguments after list command")
			}
			return nil
		default:
			return fmt.Errorf("unknown arguments to team command: %v", args)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callingDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting current workdir: %w", err)
			os.Exit(1)
		}

		repo := openRepo(cmd, callingDir)

		client, err := newClient(ctx, repo)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		subCmd := args[0]

		switch subCmd {
		case "add":
			err := client.AddRepoCollaborator(ctx, repo, args[1:])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			fmt.Printf("Added:\n%s\n", strings.Join(args[1:], "\n"))
		case "list":
			collaborators, err := client.ListRepoCollaborators(ctx, repo)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			fmt.Printf("Current collaborators:\n%s\n", strings.Join(collaborators, "\n"))
		case "remove":
			err := client.RemoveRepoCollaborator(ctx, repo, args[1:])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			fmt.Printf("Removed collaborators:\n%s\n", strings.Join(args[1:], "\n"))
		}
	},
}
