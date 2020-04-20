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

	name, _, err := chainTree.ChainTree.Dag.Resolve(ctx, []string{"tree", "data", "dgit", "name"})
	if err != nil {
		return nil, err
	}
	if name == nil {
		return nil, fmt.Errorf("Invalid dgit ChainTree, must have dgit/name set")
	}
	nameStr, ok := name.(string)
	if !ok {
		return nil, fmt.Errorf("Invalid dgit ChainTree, dgit/name must be a string, got %T", name)
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
	chainTree, err := consensus.NewSignedChainTree(ctx, opts.Key.PublicKey, opts.Tupelo.DagStore())
	if err != nil {
		return nil, err
	}

	owners := opts.Owners
	if len(owners) == 0 {
		owners = []string{crypto.PubkeyToAddress(opts.Key.PublicKey).String()}
	}

	setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction(opts.Owners)
	if err != nil {
		return nil, err
	}

	nameTxn, err := chaintree.NewSetDataTransaction("dgit/name", opts.Name)
	if err != nil {
		return nil, err
	}

	creationTimestampTxn, err := chaintree.NewSetDataTransaction("dgit/createdAt", time.Now().Unix())
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

	_, err = opts.Tupelo.PlayTransactions(ctx, chainTree, opts.Key, txns)
	if err != nil {
		return nil, err
	}

	return New(opts.Name, chainTree, opts.Tupelo), nil
}
