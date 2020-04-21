package repotree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/teamtree"
	"github.com/quorumcontrol/dgit/tupelo/tree"
	"github.com/quorumcontrol/dgit/tupelo/usertree"
)

const (
	DefaultObjectStorageType = "siaskynet"
)

var log = logging.Logger("decentragit.repotree")

var ErrNotFound = tree.ErrNotFound

var teamsMapPath = []string{"teams"}

type Options struct {
	Name              string
	Tupelo            *tupelo.Client
	Owners            []string
	ObjectStorageType string
}

type RepoTree struct {
	*tree.Tree
}

func Find(ctx context.Context, repo string, client *tupelo.Client) (*RepoTree, error) {
	log.Debugf("looking for repo %s", repo)

	username := strings.Split(repo, "/")[0]
	reponame := strings.Join(strings.Split(repo, "/")[1:], "/")

	userTree, err := usertree.Find(ctx, username, client)
	if err != nil {
		return nil, err
	}
	log.Debugf("user chaintree found for %s - %s", username, userTree.Did())

	userRepos, err := userTree.Repos(ctx)
	if err != nil {
		return nil, err
	}

	repoDid, ok := userRepos[reponame]
	if !ok || repoDid == "" {
		return nil, ErrNotFound
	}

	t, err := tree.Find(ctx, client, repoDid)
	if err != nil {
		return nil, err
	}
	log.Debugf("repo chaintree found for %s - %s", repo, t.Did())

	return &RepoTree{t}, nil
}

func Create(ctx context.Context, opts *Options, ownerKey *ecdsa.PrivateKey) (*RepoTree, error) {
	log.Debugf("creating new repotree with options: %+v", opts)

	username := strings.Split(opts.Name, "/")[0]
	reponame := strings.Join(strings.Split(opts.Name, "/")[1:], "/")

	userTree, err := usertree.Find(ctx, username, opts.Tupelo)
	if err == usertree.ErrNotFound {
		return nil, fmt.Errorf("user %s does not exist (%w)", username, err)
	}
	if err != nil {
		return nil, err
	}
	log.Debugf("user chaintree found for %s - %s", username, userTree.Did())

	isOwner, err := userTree.IsOwner(ctx, crypto.PubkeyToAddress(ownerKey.PublicKey).String())
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, fmt.Errorf("can not create repo %s, current user is not an owner of %s", opts.Name, username)
	}

	userRepos, err := userTree.Repos(ctx)
	if err != nil {
		return nil, err
	}

	_, ok := userRepos[reponame]
	if ok {
		return nil, fmt.Errorf("repo %s already exists for %s", reponame, username)
	}

	if storage, found := os.LookupEnv("DGIT_OBJ_STORAGE"); found {
		log.Warningf("[DEPRECATION] - DGIT_OBJ_STORAGE is deprecated, please use DG_OBJ_STORAGE")
		opts.ObjectStorageType = storage
	}
	if storage, found := os.LookupEnv("DG_OBJ_STORAGE"); found {
		opts.ObjectStorageType = storage
	}
	if opts.ObjectStorageType == "" {
		opts.ObjectStorageType = DefaultObjectStorageType
	}

	config := map[string]map[string]string{"objectStorage": {"type": opts.ObjectStorageType}}
	configTxn, err := chaintree.NewSetDataTransaction("config", config)
	if err != nil {
		return nil, err
	}

	log.Debugf("using object storage type %s", opts.ObjectStorageType)

	defaultTeam, err := teamtree.Create(ctx, &teamtree.Options{
		Name:   opts.Name + " default team",
		Tupelo: opts.Tupelo,
		Members: teamtree.Members{
			userTree,
		},
	})
	if err != nil {
		return nil, err
	}
	teamTxn, err := chaintree.NewSetDataTransaction(strings.Join(append(teamsMapPath, "default"), "/"), defaultTeam.Did())
	if err != nil {
		return nil, err
	}

	t, err := tree.Create(ctx, &tree.Options{
		Name:   reponame,
		Tupelo: opts.Tupelo,
		Owners: []string{
			defaultTeam.Did(),
		},
		AdditionalTxns: []*transactions.Transaction{configTxn, teamTxn},
	})
	if err != nil {
		return nil, err
	}
	log.Infof("created %s repo chaintree with did: %s", t.Name(), t.Did())

	err = userTree.AddRepo(ctx, ownerKey, reponame, t.Did())
	if err != nil {
		return nil, err
	}

	return &RepoTree{t}, nil
}

func (t *RepoTree) Team(ctx context.Context, name string) (*teamtree.TeamTree, error) {
	path := append(append([]string{"tree", "data"}, teamsMapPath...), name)
	valUncast, _, err := t.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	if valUncast == nil {
		return nil, teamtree.ErrNotFound
	}
	teamDid, ok := valUncast.(string)
	if !ok {
		return nil, fmt.Errorf("team %s is not a did string, got %T", name, valUncast)
	}
	return teamtree.Find(ctx, t.Tupelo(), teamDid)
}
