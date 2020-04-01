package repotree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/namedtree"
)

const (
	repoSalt                 = "decentragit-0.0.0-alpha"
	DefaultObjectStorageType = "siaskynet"
)

var collabPath = []string{"tree", "data", "dgit", "team"}

var namedTreeGen *namedtree.Generator

func init() {
	namedTreeGen = &namedtree.Generator{Namespace: repoSalt}
}

type Options struct {
	*namedtree.Options
}

type RepoTree struct {
	*namedtree.NamedTree
}

func Find(ctx context.Context, repo string, client *tupelo.Client) (*RepoTree, error) {
	namedTreeGen.Client = client
	nt, err := namedTreeGen.Find(ctx, repo)
	if err != nil {
		return nil, err
	}
	return &RepoTree{nt}, nil
}

func Create(ctx context.Context, opts *Options) (*RepoTree, error) {
	namedTree, err := namedTreeGen.New(ctx, opts.Options)
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

	teamTxn, err := createCollabPathTxn()
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{repoTxn, configTxn, teamTxn}

	ownerKey, err := crypto.ToECDSA([]byte(opts.Owners[0]))
	if err != nil {
		return nil, err
	}

	_, err = opts.Tupelo.PlayTransactions(ctx, namedTree.ChainTree, ownerKey, txns)
	if err != nil {
		return nil, err
	}

	return &RepoTree{namedTree}, nil
}

// updateCollaborators takes an ownerKey for the update transaction (if needed)
// and a newCollaborators func that takes the current set and returns the entire set
// of new collaborators (it is expected to close over the incoming set).
func (rt *RepoTree) updateCollaborators(ctx context.Context, ownerKey *ecdsa.PrivateKey, newCollaborators func([]string) []string) error {
	currentCollaborators, err := rt.ListCollaborators(ctx, ownerKey)
	if err != nil {
		return err
	}

	updated := newCollaborators(currentCollaborators)

	if len(updated) == len(currentCollaborators) {
		return nil
	}

	txn, err := chaintree.NewSetDataTransaction("dgit/team", updated)
	if err != nil {
		return err
	}

	_, err = rt.Tupelo.PlayTransactions(ctx, rt.ChainTree, ownerKey, []*transactions.Transaction{txn})
	if err != nil {
		return err
	}

	return nil
}

func (rt *RepoTree) AddCollaborators(ctx context.Context, collaborators []string, ownerKey *ecdsa.PrivateKey) error {
	return rt.updateCollaborators(ctx, ownerKey, func(current []string) []string {
		var newCollaborators []string

		for _, c := range collaborators {
			var exists bool
			for _, cc := range current {
				if c == cc {
					exists = true
					break
				}
			}
			if !exists {
				newCollaborators = append(current, c)
			}
		}

		return newCollaborators
	})
}

func createCollabPathTxn() (*transactions.Transaction, error) {
	return chaintree.NewSetDataTransaction(strings.Join(collabPath[2:], "/"), []string{})
}

func (rt *RepoTree) createCollabPath(ctx context.Context, ownerKey *ecdsa.PrivateKey) error {
	txn, err := createCollabPathTxn()
	if err != nil {
		return err
	}

	_, err = rt.Tupelo.PlayTransactions(ctx, rt.ChainTree, ownerKey, []*transactions.Transaction{txn})
	if err != nil {
		return err
	}

	return nil
}

func (rt *RepoTree) ListCollaborators(ctx context.Context, ownerKey *ecdsa.PrivateKey) ([]string, error) {
	collaboratorsUncast, remaining, err := rt.ChainTree.ChainTree.Dag.Resolve(ctx, collabPath)
	if len(remaining) > 0 {
		err = rt.createCollabPath(ctx, ownerKey)
		return []string{}, err
	}
	if err != nil {
		return []string{}, err
	}

	var (
		collabsSemiCast []interface{}
		ok              bool
	)

	collaborators := make([]string, 0)

	if collabsSemiCast, ok = collaboratorsUncast.([]interface{}); !ok {
		return []string{}, fmt.Errorf("collaborators value was not an interface{} slice; was a %T: %+v", collaboratorsUncast, collaboratorsUncast)
	}
	for _, c := range collabsSemiCast {
		var cStr string
		if cStr, ok = c.(string); !ok {
			return []string{}, fmt.Errorf("collaborator value was not a string; was a %T: %+v", c, c)
		}
		collaborators = append(collaborators, cStr)
	}

	return collaborators, nil
}

func (rt *RepoTree) RemoveCollaborators(ctx context.Context, collaborators []string, ownerKey *ecdsa.PrivateKey) error {
	return rt.updateCollaborators(ctx, ownerKey, func(current []string) []string {
		newCollaborators := make([]string, 0)

		for _, cc := range current {
			var remove bool
			for _, c := range collaborators {
				if cc == c {
					remove = true
					break
				}
			}
			if !remove {
				newCollaborators = append(newCollaborators, cc)
			}
		}

		return newCollaborators
	})
}
