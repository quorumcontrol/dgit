package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/quorumcontrol/dgit/remotehelper"
	"github.com/quorumcontrol/dgit/storage/readonly"
	"github.com/quorumcontrol/dgit/storage/split"
	"github.com/quorumcontrol/dgit/transport/dgit"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(remoteHelperCommand)
}

var remoteHelperCommand = &cobra.Command{
	Use:    "remote-helper",
	Short:  "A git-remote-helper called by git directly. Not for direct use!",
	Long:   `Implements a git-remote-helper (https://git-scm.com/docs/git-remote-helpers), registering and handling the dg:// protocol.`,
	Args:   cobra.ArbitraryArgs,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		gitStore := filesystem.NewStorage(osfs.New(os.Getenv("GIT_DIR")), cache.NewObjectLRUDefault())
		readonlyStore := readonly.NewStorage(gitStore)

		// git-remote-helper expects this script to write git objects, but nothing else
		// therefore initialize a go-git storage with the ability to write objects & shallow
		// but make reference, index, and config read only ops
		storer := split.NewStorage(&split.StorageMap{
			ObjectStorage:    gitStore,
			ShallowStorage:   gitStore,
			ReferenceStorage: readonlyStore,
			IndexStorage:     readonlyStore,
			ConfigStorage:    readonlyStore,
		})

		local, err := git.Open(storer, nil)
		if err != nil {
			panic(err)
		}

		r := remotehelper.New(local)

		log.Infof("decentragit remote helper loaded for %s", os.Getenv("GIT_DIR"))

		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: git-remote-dg <remote-name> <url>")
			os.Exit(1)
		}

		client, err := dgit.NewClient(ctx, os.Getenv("GIT_DIR"))
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("error starting decentragit client: %v", err))
			os.Exit(1)
		}
		client.RegisterAsDefault()

		if err := r.Run(ctx, args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}
