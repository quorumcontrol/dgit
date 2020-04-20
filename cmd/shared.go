package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/spf13/cobra"

	"github.com/quorumcontrol/dgit/msg"
	"github.com/quorumcontrol/dgit/transport/dgit"
)

func openRepo(cmd *cobra.Command, path string) *dgit.Repo {
	gitRepo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit: true,
	})

	if err == git.ErrRepositoryNotExists {
		msg.Fprint(os.Stderr, msg.RepoNotFoundInPath, map[string]interface{}{
			"path": path,
			"cmd":  rootCmd.Name() + " " + cmd.Name(),
		})
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return dgit.NewRepo(gitRepo)
}

func newClient(ctx context.Context, repo *dgit.Repo) (*dgit.Client, error) {
	repoGitPath := repo.Storer.(*filesystem.Storage).Filesystem().Root()

	client, err := dgit.NewClient(ctx, repoGitPath)
	if err != nil {
		return nil, fmt.Errorf("error starting decentragit client: %w", err)
	}
	client.RegisterAsDefault()

	return client, nil
}
