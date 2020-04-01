package cmd

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/filesystem"

	"github.com/quorumcontrol/dgit/transport/dgit"
)

func openRepo(path string) (*dgit.Repo, error) {
	gitRepo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit: true,
	})

	if err != nil {
		return nil, err
	}

	return &dgit.Repo{Repository: gitRepo}, nil
}

func newClient(ctx context.Context, repo *dgit.Repo) (*dgit.Client, error) {
	repoGitPath := repo.Storer.(*filesystem.Storage).Filesystem().Root()

	client, err := dgit.NewClient(ctx, repoGitPath)
	if err != nil {
		return nil, fmt.Errorf("error starting dgit client: %w", err)
	}
	client.RegisterAsDefault()

	return client, nil
}
