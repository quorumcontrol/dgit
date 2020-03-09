package dgit

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/decentragit-remote/storage/chaintree"
	"github.com/quorumcontrol/decentragit-remote/tupelo/repotree"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/server"
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
	chainTree, err := repotree.Find(l.ctx, ep.Host+"/"+ep.Path, l.tupelo)

	var privateKey *ecdsa.PrivateKey

	switch auth := l.auth.(type) {
	case *PrivateKeyAuth:
		privateKey = auth.Key()
	case nil:
		// noop
	default:
		return nil, fmt.Errorf("Unsupported auth type %T", l.auth)
	}

	if err == repotree.ErrNotFound {
		return nil, transport.ErrRepositoryNotFound
	}

	if err != nil {
		return nil, err
	}

	return chaintree.NewStorage(&chaintree.StorageConfig{
		Ctx:        l.ctx,
		Tupelo:     l.tupelo,
		ChainTree:  chainTree,
		PrivateKey: privateKey,
	}), nil
}
