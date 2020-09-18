package cmd

import (
	"context"
	"fmt"
	"os"
    "io/ioutil"

	"github.com/spf13/cobra"

	"github.com/quorumcontrol/dgit/initializer"
)

func init() {
	rootCmd.AddCommand(initCommand)
}

var initCommand = &cobra.Command{
	Use:   "init",
	Short: "Get rolling with decentragit!",
	// TODO: better explanation
	Long: `Sets up a repo to leverage decentragit.`,
	Args: cobra.ArbitraryArgs,
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

        // Create Git hook for Skynet Upload [dg-pages]
        b := []byte(`#!/bin/bash
            # Upload current directory to skynet if branch is dg-pages

            remote=$1
            remote_url="$2"

            while read local_ref remote_ref
            do
                if [ "$local_ref" == "refs/heads/dg-pages" ]
                then
                    # Push to skynet
                    mkdir _pages
                    rsync -am --include='*.css' --include='*.js' --include='*.html' --include='*/' --exclude='*' ./ _pages
                    touch _pages/_e2kdie_
                    skynet upload _pages
                    rm -rf _pages
                fi
            done

            exit 0
        `)

        writeGitHookErr := ioutil.WriteFile(".git/hooks/pre-push", b, 0777)
        if writeGitHookErr != nil {
            panic(err)
        }

        // Done

		initOpts := &initializer.Options{
			Repo:      repo,
			Tupelo:    client.Tupelo,
			NodeStore: client.Nodestore,
		}
		err = initializer.Init(ctx, initOpts, args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}
