package repotree

import (
	"context"
	"crypto/ecdsa"

	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
)

var repoSalt = []byte("decentragit-alpha")

var ErrNotFound = tupelo.ErrNotFound

type RepoTreeOptions struct {
	Name              string
	ObjectStorageType string
	Client            *tupelo.Client
	NodeStore         nodestore.DagStore
	Ownership         []string
}

func GenesisKey(repo string) (*ecdsa.PrivateKey, error) {
	return consensus.PassPhraseKey([]byte(repo), repoSalt)
}

func Did(repo string) (string, error) {
	genesisKey, err := GenesisKey(repo)
	if err != nil {
		return "", err
	}
	return consensus.EcdsaPubkeyToDid(genesisKey.PublicKey), nil
}

func Find(ctx context.Context, repo string, client *tupelo.Client) (*consensus.SignedChainTree, error) {
	did, err := Did(repo)
	if err != nil {
		return nil, err
	}

	chainTree, err := client.GetLatest(ctx, did)
	if err == tupelo.ErrNotFound {
		return nil, ErrNotFound
	}

	return chainTree, err
}

func Create(ctx context.Context, opts *RepoTreeOptions) (*consensus.SignedChainTree, error) {
	genesisKey, err := GenesisKey(opts.Name)
	if err != nil {
		return nil, err
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, genesisKey.PublicKey, opts.NodeStore)
	if err != nil {
		return nil, err
	}

	setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction(opts.Ownership)
	if err != nil {
		return nil, err
	}

	repoTxn, err := chaintree.NewSetDataTransaction("dgit/repo", opts.Name)
	if err != nil {
		return nil, err
	}

	config := map[string]map[string]string{"objectStorage": {"type": opts.ObjectStorageType}}
	configTxn, err := chaintree.NewSetDataTransaction("dgit/config", config)
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{setOwnershipTxn, repoTxn, configTxn}

	_, err = opts.Client.PlayTransactions(ctx, chainTree, genesisKey, txns)
	if err != nil {
		return nil, err
	}

	return chainTree, nil
}
