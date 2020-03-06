package chaintree

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/quorumcontrol/decentragit-remote/storage/split"
	"github.com/quorumcontrol/decentragit-remote/tupelo/clientbuilder"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/src-d/go-git.v4/storage/test"
)

var objectTests = []string{
	"TestSetEncodedObjectAndEncodedObject",
	"TestSetEncodedObjectInvalid",
	"TestIterEncodedObjects",
	"TestPackfileWriter",
	"TestObjectStorerTxSetEncodedObjectAndCommit",
	"TestObjectStorerTxSetObjectAndGetObject",
	"TestObjectStorerTxGetObjectNotFound",
	"TestObjectStorerTxSetObjectAndRollback",
}

func Test(t *testing.T) {
	testsRegex := "^(" + strings.Join(objectTests, "|") + ")$"
	result := RunAll(&RunConf{
		Filter: testsRegex,
	})
	println(result.String())
	if !result.Passed() {
		t.Fail()
	}
}

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

	objectStore := NewObjectStorage(context.Background(), tupelo, chainTree, key)

	// split store here is just because base expects it,
	// but only run object tests as specified above
	splitStore := split.NewStorage(&split.StorageMap{
		ObjectStorage:    objectStore,
		ShallowStorage:   memory.NewStorage(),
		ReferenceStorage: memory.NewStorage(),
		IndexStorage:     memory.NewStorage(),
		ConfigStorage:    memory.NewStorage(),
	})

	s.BaseStorageSuite = test.NewBaseStorageSuite(splitStore)
	s.BaseStorageSuite.SetUpTest(c)
}
