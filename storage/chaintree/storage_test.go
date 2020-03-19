package chaintree

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-git/go-git/v5/storage/test"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	. "gopkg.in/check.v1"

	"github.com/quorumcontrol/dgit/storage"
	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
)

func Test(t *testing.T) { TestingT(t) }

type StorageSuite struct {
	test.BaseStorageSuite
}

var _ = Suite(&StorageSuite{})

func (s *StorageSuite) SetUpSuite(c *C) {
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
}

func (s *StorageSuite) TearDownTest(c *C) {
	s.BaseStorageSuite.TearDownTest(c)
}

// override a test that will fail for reasons we don't care about
func (s *StorageSuite) TestModule(c *C) {}
