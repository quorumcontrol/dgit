package namedtree

import (
	"context"
	"crypto/ecdsa"
	"strings"

	"github.com/quorumcontrol/dgit/tupelo/tree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo/sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"
)

var ErrNotFound = tree.ErrNotFound

type Generator struct {
	Namespace string
	Client    *tupelo.Client
}

type NamedTree struct {
	*tree.Tree
}

type Options struct {
	Name           string
	Tupelo         *tupelo.Client
	Owners         []string
	AdditionalTxns []*transactions.Transaction
}

func (g *Generator) GenesisKey(name string) (*ecdsa.PrivateKey, error) {
	return consensus.PassPhraseKey([]byte(name), []byte(g.Namespace))
}

func (g *Generator) Create(ctx context.Context, opts *Options) (*NamedTree, error) {
	gKey, err := g.GenesisKey(opts.Name)
	if err != nil {
		return nil, err
	}

	t, err := tree.Create(ctx, &tree.Options{
		Name:           opts.Name,
		Key:            gKey,
		Owners:         opts.Owners,
		Tupelo:         opts.Tupelo,
		AdditionalTxns: opts.AdditionalTxns,
	})
	if err != nil {
		return nil, err
	}

	return &NamedTree{t}, nil
}

// Did lower-cases the name arg first to ensure that chaintree
// names are case insensitive. If we ever want case sensitivity,
// consider adding a bool flag to the Generator or adding a new
// fun.
func (g *Generator) Did(name string) (string, error) {
	gKey, err := g.GenesisKey(strings.ToLower(name))
	if err != nil {
		return "", err
	}

	return consensus.EcdsaPubkeyToDid(gKey.PublicKey), nil
}

func (g *Generator) Find(ctx context.Context, name string) (*NamedTree, error) {
	did, err := g.Did(name)
	if err != nil {
		return nil, err
	}

	t, err := tree.Find(ctx, g.Client, did)
	if err != nil {
		return nil, err
	}

	return &NamedTree{t}, nil
}
