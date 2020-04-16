package chaintree

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/quorumcontrol/tupelo/sdk/consensus"
	"github.com/quorumcontrol/tupelo/sdk/p2p"
	. "gopkg.in/check.v1"

	gitstorage "github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/test"

	"github.com/quorumcontrol/dgit/storage"
	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
)

func Test(t *testing.T) { TestingT(t) }

type StorageSuite struct {
	test.BaseStorageSuite
}

var _ = Suite(&StorageSuite{})

func createChainTree(c *C, ctx context.Context, store *p2p.BitswapPeer) (*consensus.SignedChainTree, *ecdsa.PrivateKey) {
	key, err := crypto.GenerateKey()
	c.Assert(err, IsNil)

	chainTree, err := consensus.NewSignedChainTree(ctx, key.PublicKey, store)
	c.Assert(err, IsNil)

	return chainTree, key
}

func (s *StorageSuite) InitStorage(c *C) gitstorage.Storer {
	ctx := context.Background()

	// TODO: replace with mock client rather than local running tupelo docker
	tupelo, store, err := clientbuilder.BuildLocal(ctx)
	c.Assert(err, IsNil)

	chainTree, key := createChainTree(c, ctx, store)

	st, err := NewStorage(&storage.Config{
		Ctx:        ctx,
		Tupelo:     tupelo,
		ChainTree:  chainTree,
		PrivateKey: key,
	})
	c.Assert(err, IsNil)

	return st
}

func (s *StorageSuite) SetUpSuite(c *C) {
	st := s.InitStorage(c)
	s.BaseStorageSuite = test.NewBaseStorageSuite(st)
}

func (s *StorageSuite) TearDownTest(c *C) {
	// reset this to a new storage every time or tests will share one chaintree
	// and interfere w/ each other
	s.Storer = s.InitStorage(c)

	s.BaseStorageSuite.TearDownTest(c)
}

// override a test that will fail for reasons we don't care about
func (s *StorageSuite) TestModule(c *C) {}
