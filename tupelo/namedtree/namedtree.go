package namedtree

import (
	"context"
	"crypto/ecdsa"
	"os"
	"strings"
	"time"

	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
)

const DefaultObjectStorageType = "siaskynet"

var ErrNotFound = tupelo.ErrNotFound

type Generator struct {
	Namespace string
	Client    *tupelo.Client
}

type NamedTree struct {
	Name      string
	ChainTree *consensus.SignedChainTree

	genesisKey *ecdsa.PrivateKey
	client     *tupelo.Client
	nodeStore  nodestore.DagStore
	owners     []string
}

type Options struct {
	Name              string
	ObjectStorageType string
	Client            *tupelo.Client
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

	repoTxn, err := chaintree.NewSetDataTransaction("dgit/repo", opts.Name)
	if err != nil {
		return nil, err
	}

	if storage, found := os.LookupEnv("DGIT_OBJ_STORAGE"); found {
		opts.ObjectStorageType = storage
	}
	if opts.ObjectStorageType == "" {
		opts.ObjectStorageType = DefaultObjectStorageType
	}
	config := map[string]map[string]string{"objectStorage": {"type": opts.ObjectStorageType}}
	configTxn, err := chaintree.NewSetDataTransaction("dgit/config", config)
	if err != nil {
		return nil, err
	}

	creationTimestampTxn, err := chaintree.NewSetDataTransaction("dgit/createdAt", time.Now().Unix())
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{setOwnershipTxn, repoTxn, configTxn, creationTimestampTxn}

	_, err = opts.Client.PlayTransactions(ctx, chainTree, gKey, txns)
	if err != nil {
		return nil, err
	}

	return &NamedTree{
		Name:       opts.Name,
		ChainTree:  chainTree,
		genesisKey: gKey,
		client:     opts.Client,
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
		client:     g.Client,
	}, nil
}

func (t *NamedTree) Did() string {
	return consensus.EcdsaPubkeyToDid(t.genesisKey.PublicKey)
}
