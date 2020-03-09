package split

import (
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/src-d/go-git.v4/storage/test"
)

func Test(t *testing.T) { TestingT(t) }

type StorageSuite struct {
	test.BaseStorageSuite
}

var _ = Suite(&StorageSuite{})

func (s *StorageSuite) SetUpTest(c *C) {
	splitStore := NewStorage(&StorageMap{
		ObjectStorage:    memory.NewStorage(),
		ShallowStorage:   memory.NewStorage(),
		ReferenceStorage: memory.NewStorage(),
		IndexStorage:     memory.NewStorage(),
		ConfigStorage:    memory.NewStorage(),
	})
	s.BaseStorageSuite = test.NewBaseStorageSuite(splitStore)
	s.BaseStorageSuite.SetUpTest(c)
}
