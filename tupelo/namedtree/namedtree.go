package namedtree

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"time"

	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
)

var ErrNotFound = tupelo.ErrNotFound

type Generator struct {
	Namespace string
	Client    *tupelo.Client
}

type NamedTree struct {
	Name      string
	ChainTree *consensus.SignedChainTree
	Tupelo    *tupelo.Client

	genesisKey *ecdsa.PrivateKey
	nodeStore  nodestore.DagStore
	owners     []string
}

type Options struct {
	Name              string
	ObjectStorageType string
	Tupelo            *tupelo.Client
	NodeStore         nodestore.DagStore
	Owners            []string
}

func (g *Generator) GenesisKey(name string) (*ecdsa.PrivateKey, error) {
	return consensus.PassPhraseKey([]byte(name), []byte(g.Namespace))
}

func (g *Generator) New(ctx context.Context, opts *Options) (*NamedTree, error) {
	gKey, err := g.GenesisKey(opts.Name)
	if err != nil {
		return nil, err
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, gKey.PublicKey, opts.NodeStore)
	if err != nil {
		return nil, err
	}

	setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction(opts.Owners)
	if err != nil {
		return nil, err
	}

	creationTimestampTxn, err := chaintree.NewSetDataTransaction("dgit/createdAt", time.Now().Unix())
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{setOwnershipTxn, creationTimestampTxn}

	_, err = opts.Tupelo.PlayTransactions(ctx, chainTree, gKey, txns)
	if err != nil {
		return nil, err
	}

	return &NamedTree{
		Name:       opts.Name,
		ChainTree:  chainTree,
		genesisKey: gKey,
		Tupelo:     opts.Tupelo,
		nodeStore:  opts.NodeStore,
		owners:     opts.Owners,
	}, nil
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

	chainTree, err := g.Client.GetLatest(ctx, did)
	if err == tupelo.ErrNotFound {
		return nil, ErrNotFound
	}

	gKey, err := g.GenesisKey(name)
	if err != nil {
		return nil, err
	}

	return &NamedTree{
		Name:       name,
		ChainTree:  chainTree,
		genesisKey: gKey,
		Tupelo:     g.Client,
	}, nil
}

func (t *NamedTree) Did() string {
	return consensus.EcdsaPubkeyToDid(t.genesisKey.PublicKey)
}
