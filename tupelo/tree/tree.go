package tree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo/sdk/consensus"
	"github.com/quorumcontrol/tupelo/sdk/gossip/client"
)

var ErrNotFound = client.ErrNotFound

type Tree struct {
	name      string
	chainTree *consensus.SignedChainTree
	tupelo    *client.Client
}

type Options struct {
	Name           string
	Tupelo         *client.Client
	Owners         []string
	Key            *ecdsa.PrivateKey
	AdditionalTxns []*transactions.Transaction
}

func (t *Tree) Name() string {
	return t.name
}

func (t *Tree) ChainTree() *consensus.SignedChainTree {
	return t.chainTree
}

func (t *Tree) Did() string {
	return t.chainTree.MustId()
}

func (t *Tree) Tupelo() *client.Client {
	return t.tupelo
}

func (t *Tree) Resolve(ctx context.Context, path []string) (interface{}, []string, error) {
	return t.ChainTree().ChainTree.Dag.Resolve(ctx, path)
}

func Find(ctx context.Context, tupelo *client.Client, did string) (*Tree, error) {
	chainTree, err := tupelo.GetLatest(ctx, did)
	if err == client.ErrNotFound {
		return nil, ErrNotFound
	}

	name, _, err := chainTree.ChainTree.Dag.Resolve(ctx, []string{"tree", "data", "name"})
	if err != nil {
		return nil, err
	}
	if name == nil {
		return nil, fmt.Errorf("Invalid dgit ChainTree, must have .name set")
	}
	nameStr, ok := name.(string)
	if !ok {
		return nil, fmt.Errorf("Invalid dgit ChainTree, .name must be a string, got %T", name)
	}

	return New(nameStr, chainTree, tupelo), nil
}

func New(name string, chainTree *consensus.SignedChainTree, tupelo *client.Client) *Tree {
	return &Tree{
		name:      name,
		chainTree: chainTree,
		tupelo:    tupelo,
	}
}

func Create(ctx context.Context, opts *Options) (*Tree, error) {
	var err error

	key := opts.Key
	if key == nil {
		key, err = crypto.GenerateKey()
		if err != nil {
			return nil, err
		}
	}

	owners := opts.Owners
	if len(owners) == 0 {
		owners = []string{crypto.PubkeyToAddress(key.PublicKey).String()}
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, key.PublicKey, opts.Tupelo.DagStore())
	if err != nil {
		return nil, err
	}

	setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction(owners)
	if err != nil {
		return nil, err
	}

	nameTxn, err := chaintree.NewSetDataTransaction("name", opts.Name)
	if err != nil {
		return nil, err
	}

	creationTimestampTxn, err := chaintree.NewSetDataTransaction("createdAt", time.Now().Unix())
	if err != nil {
		return nil, err
	}

	docTypeTxn, err := chaintree.NewSetDataTransaction("__doctype", "dgit")
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{setOwnershipTxn, nameTxn, creationTimestampTxn, docTypeTxn}

	if opts.AdditionalTxns != nil {
		txns = append(txns, opts.AdditionalTxns...)
	}

	_, err = opts.Tupelo.PlayTransactions(ctx, chainTree, key, txns)
	if err != nil {
		return nil, err
	}

	return New(opts.Name, chainTree, opts.Tupelo), nil
}
