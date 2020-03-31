package usertree

import (
	"context"

	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/namedtree"
)

const userSalt = "dgit-user-v0"

var namedTreeGen *namedtree.Generator

func init() {
	namedTreeGen = &namedtree.Generator{Namespace: userSalt}
}

type Options struct {
	*namedtree.Options
}

func Find(ctx context.Context, username string, client *tupelo.Client) (*namedtree.NamedTree, error) {
	namedTreeGen.Client = client
	return namedTreeGen.Find(ctx, username)
}

func Create(ctx context.Context, opts *Options) (*namedtree.NamedTree, error) {
	namedTree, err := namedTreeGen.New(ctx, opts.Options)
	if err != nil {
		return nil, err
	}

	return namedTree, nil
}
