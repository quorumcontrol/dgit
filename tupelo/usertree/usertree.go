package usertree

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/namedtree"
)

type UserTree struct {
	*namedtree.NamedTree
}

var log = logging.Logger("dgit.usertree")

const userSalt = "dgit-user-v0"

var namedTreeGen *namedtree.Generator

var reposMapPath = []string{"dgit", "repos"}

var ErrNotFound = tupelo.ErrNotFound

func init() {
	namedTreeGen = &namedtree.Generator{Namespace: userSalt}
}

type Options namedtree.Options

func Find(ctx context.Context, username string, client *tupelo.Client) (*UserTree, error) {
	namedTreeGen.Client = client
	namedTree, err := namedTreeGen.Find(ctx, username)
	if err == namedtree.ErrNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &UserTree{namedTree}, nil
}

func Create(ctx context.Context, opts *Options) (*UserTree, error) {
	ntOpts := namedtree.Options(*opts)
	namedTree, err := namedTreeGen.New(ctx, &ntOpts)
	if err != nil {
		return nil, err
	}

	log.Debugf("created user %s (%s)", opts.Name, namedTree.Did())

	return &UserTree{namedTree}, nil
}

func (t *UserTree) IsOwner(ctx context.Context, addr string) (bool, error) {
	auths, err := t.ChainTree.Authentications()
	if err != nil {
		return false, err
	}
	log.Debugf("checking %s is owner of %s, chaintree auths: %v", addr, t.ChainTree.MustId(), auths)

	for _, auth := range auths {
		if auth == addr {
			return true, nil
		}
	}

	return false, nil
}

func (t *UserTree) AddRepo(ctx context.Context, ownerKey *ecdsa.PrivateKey, reponame string, did string) error {
	log.Debugf("adding repo %s (%s) to user %s (%s)", reponame, did, t.Name, t.Did())

	path := strings.Join(append(reposMapPath, reponame), "/")
	repoTxn, err := chaintree.NewSetDataTransaction(path, did)
	if err != nil {
		return err
	}

	_, err = t.Tupelo.PlayTransactions(ctx, t.ChainTree, ownerKey, []*transactions.Transaction{repoTxn})
	return err
}

func (t *UserTree) Repos(ctx context.Context) (map[string]string, error) {
	path := append([]string{"tree", "data"}, reposMapPath...)
	valMap := make(map[string]string)
	valUncast, _, err := t.ChainTree.ChainTree.Dag.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	if valUncast == nil {
		return valMap, nil
	}

	valMapUncast, ok := valUncast.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("path %v is %T, expected map", path, valUncast)
	}

	for k, v := range valMapUncast {
		if v == nil {
			continue
		}
		vstr, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("key %s at path %v is %T, expected string", k, path, v)
		}
		valMap[k] = vstr
	}

	return valMap, nil
}
