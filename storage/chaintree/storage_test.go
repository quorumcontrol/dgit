package chaintree

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/storage/test"

	"github.com/quorumcontrol/dgit/storage"
	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
)

func Test(t *testing.T) { TestingT(t) }

type StorageSuite struct {
	test.BaseStorageSuite
}

var _ = Suite(&StorageSuite{})

func (s *StorageSuite) SetUpTest(c *C) {
	ctx := context.Background()

	// TODO: replace with mock client rather than local running tupelo docker
	tupelo, store, err := clientbuilder.BuildLocal(ctx)
	c.Assert(err, IsNil)

	key, err := crypto.GenerateKey()
	c.Assert(err, IsNil)

	chainTree, err := consensus.NewSignedChainTree(ctx, key.PublicKey, store)
	c.Assert(err, IsNil)

	st, err := NewStorage(&storage.Config{
		Ctx:        ctx,
		Tupelo:     tupelo,
		ChainTree:  chainTree,
		PrivateKey: key,
	})
	c.Assert(err, IsNil)

	s.BaseStorageSuite = test.NewBaseStorageSuite(st)
	s.BaseStorageSuite.SetUpTest(c)
}
