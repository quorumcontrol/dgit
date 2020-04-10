package repotree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/namedtree"
	"github.com/quorumcontrol/dgit/tupelo/usertree"
)

const (
	repoSalt                 = "decentragit-0.0.0-alpha"
	DefaultObjectStorageType = "siaskynet"
)

var log = logging.Logger("dgit.repotree")

var collabPath = []string{"tree", "data", "dgit", "team"}

var namedTreeGen *namedtree.Generator

var ErrNotFound = tupelo.ErrNotFound

func init() {
	namedTreeGen = &namedtree.Generator{Namespace: repoSalt}
}

type Options namedtree.Options

type RepoTree struct {
	Name      string
	ChainTree *consensus.SignedChainTree
	Tupelo    *tupelo.Client
}

func Find(ctx context.Context, repo string, client *tupelo.Client) (*RepoTree, error) {
	log.Debugf("looking for repo %s", repo)

	username := strings.Split(repo, "/")[0]
	reponame := strings.Join(strings.Split(repo, "/")[1:], "/")

	userChainTree, err := usertree.Find(ctx, username, client)
	if err != nil {
		return nil, err
	}
	log.Debugf("user chaintree found for %s - %s", username, userChainTree.ChainTree.MustId())

	userRepos, err := userChainTree.Repos(ctx)
	if err != nil {
		return nil, err
	}

	repoDid, ok := userRepos[reponame]
	if !ok {
		return nil, ErrNotFound
	}

	chainTree, err := client.GetLatest(ctx, repoDid)
	if err == tupelo.ErrNotFound {
		return nil, ErrNotFound
	}
	log.Debugf("repo chaintree found for %s - %s", repo, chainTree.MustId())

	return &RepoTree{
		Name:      repo,
		ChainTree: chainTree,
		Tupelo:    client,
	}, nil
}

func Create(ctx context.Context, opts *Options, ownerKey *ecdsa.PrivateKey) (*RepoTree, error) {
	log.Debugf("creating new repotree with options: %+v", opts)

	username := strings.Split(opts.Name, "/")[0]
	reponame := strings.Join(strings.Split(opts.Name, "/")[1:], "/")

	userChainTree, err := usertree.Find(ctx, username, opts.Tupelo)
	if err == usertree.ErrNotFound {
		return nil, fmt.Errorf("user %s does not exist (%w)", username, err)
	}
	if err != nil {
		return nil, err
	}
	log.Debugf("user chaintree found for %s - %s", username, userChainTree.ChainTree.MustId())

	isOwner, err := userChainTree.IsOwner(ctx, crypto.PubkeyToAddress(ownerKey.PublicKey).String())
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, fmt.Errorf("can not create repo %s, current user is not an owner of %s", opts.Name, username)
	}

	userRepos, err := userChainTree.Repos(ctx)
	if err != nil {
		return nil, err
	}

	_, ok := userRepos[reponame]
	if ok {
		return nil, fmt.Errorf("repo %s already exists for %s", reponame, username)
	}

	creationKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, creationKey.PublicKey, opts.NodeStore)
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
	log.Debugf("using object storage type %s", opts.ObjectStorageType)
	config := map[string]map[string]string{"objectStorage": {"type": opts.ObjectStorageType}}
	configTxn, err := chaintree.NewSetDataTransaction("dgit/config", config)
	if err != nil {
		return nil, err
	}

	teamTxn, err := createCollabPathTxn()
	if err != nil {
		return nil, err
	}

	txns := []*transactions.Transaction{setOwnershipTxn, creationTimestampTxn, repoTxn, configTxn, teamTxn}

	_, err = opts.Tupelo.PlayTransactions(ctx, chainTree, creationKey, txns)
	if err != nil {
		return nil, err
	}
	log.Debugf("created %s repo chaintree with did: %s", opts.Name, chainTree.MustId())

	err = userChainTree.AddRepo(ctx, ownerKey, reponame, chainTree.MustId())
	if err != nil {
		return nil, err
	}

	return &RepoTree{
		Name:      reponame,
		ChainTree: chainTree,
		Tupelo:    opts.Tupelo,
	}, nil
}

func (rt *RepoTree) usernameToKeyAddr(ctx context.Context, username string) (string, error) {
	userTree, err := usertree.Find(ctx, username, rt.Tupelo)
	if err != nil {
		return "", err
	}

	auths, err := userTree.ChainTree.Authentications()
	if err != nil {
		return "", err
	}

	return auths[0], nil
}

func (rt *RepoTree) Did() string {
	return rt.ChainTree.MustId()
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

	teamTxn, err := chaintree.NewSetDataTransaction("dgit/team", updated)
	if err != nil {
		return err
	}

	keyAddrs, err := rt.ChainTree.Authentications()
	if err != nil {
		return err
	}

	newKeyAddrs := make([]string, 0)
	for _, u := range updated {
		ka, err := rt.usernameToKeyAddr(ctx, u)
		if err != nil {
			return err
		}
		newKeyAddrs = append(newKeyAddrs, ka)
	}

	// append to first owner to ensure primary owner never changes
	keyAddrs = append([]string{keyAddrs[0]}, newKeyAddrs...)

	ownersTxn, err := chaintree.NewSetOwnershipTransaction(keyAddrs)

	_, err = rt.Tupelo.PlayTransactions(ctx, rt.ChainTree, ownerKey, []*transactions.Transaction{teamTxn, ownersTxn})
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
