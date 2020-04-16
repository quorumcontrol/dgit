package dgit

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/quorumcontrol/chaintree/nodestore"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"

	"github.com/quorumcontrol/dgit/storage"
	"github.com/quorumcontrol/dgit/storage/chaintree"
	"github.com/quorumcontrol/dgit/tupelo/namedtree"
	"github.com/quorumcontrol/dgit/tupelo/repotree"
)

// Load loads a storer.Storer given a transport.Endpoint.
// Returns transport.ErrRepositoryNotFound if the repository does not
// exist.
type ChainTreeLoader struct {
	server.Loader

	ctx       context.Context
	auth      transport.AuthMethod
	tupelo    *tupelo.Client
	nodestore nodestore.DagStore
}

func NewChainTreeLoader(ctx context.Context, tupelo *tupelo.Client, nodestore nodestore.DagStore, auth transport.AuthMethod) server.Loader {
	return &ChainTreeLoader{
		ctx:       ctx,
		tupelo:    tupelo,
		nodestore: nodestore,
		auth:      auth,
	}
}

func (l *ChainTreeLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	repoTree, err := repotree.Find(l.ctx, ep.Host+ep.Path, l.tupelo)

	var privateKey *ecdsa.PrivateKey

	switch auth := l.auth.(type) {
	case *PrivateKeyAuth:
		privateKey = auth.Key()
	case nil:
		// noop
	default:
		return nil, fmt.Errorf("Unsupported auth type %T", l.auth)
	}

	if err == namedtree.ErrNotFound {
		return nil, transport.ErrRepositoryNotFound
	}

	if err != nil {
		return nil, err
	}

	return chaintree.NewStorage(&storage.Config{
		Ctx:        l.ctx,
		Tupelo:     l.tupelo,
		ChainTree:  repoTree.ChainTree,
		PrivateKey: privateKey,
	})
}
