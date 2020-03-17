package split

import (
	"testing"

	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/storage/test"
	. "gopkg.in/check.v1"
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
}
