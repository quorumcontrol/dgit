package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/quorumcontrol/dgit/remotehelper"
	"github.com/quorumcontrol/dgit/storage/readonly"
	"github.com/quorumcontrol/dgit/storage/split"
	"github.com/quorumcontrol/dgit/transport/dgit"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

func init() {
	rootCmd.AddCommand(remoteHelperCommand)
}

var remoteHelperCommand = &cobra.Command{
	Use:    "remote-helper",
	Short:  "A git-remote-helper called by git directly. Not for direct use!",
	Long:   `Implements a git-remote-helper (https://git-scm.com/docs/git-remote-helpers), registering and handling the dgit:// protocol.`,
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

		log.Infof("dgit remote helper loaded for %s", os.Getenv("GIT_DIR"))

		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: git-remote-dgit <remote-name> <url>")
			os.Exit(1)
		}

		client, err := dgit.NewClient(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("error starting dgit client: %v", err))
			os.Exit(1)
		}
		client.RegisterAsDefault()

		if err := r.Run(ctx, args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}
