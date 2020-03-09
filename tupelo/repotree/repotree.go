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

func Create(ctx context.Context, repo string, client *tupelo.Client, store nodestore.DagStore, ownership []string) (*consensus.SignedChainTree, error) {
	genesisKey, err := GenesisKey(repo)
	if err != nil {
		return nil, err
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, genesisKey.PublicKey, store)
	if err != nil {
		return nil, err
	}

	setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction(ownership)
	if err != nil {
		return nil, err
	}

	repoTxn, err := chaintree.NewSetDataTransaction("repo", repo)
	if err != nil {
		return nil, err
	}

	transactions := []*transactions.Transaction{setOwnershipTxn, repoTxn}

	_, err = client.PlayTransactions(ctx, chainTree, genesisKey, transactions)
	if err != nil {
		return nil, err
	}

	return chainTree, nil
}
