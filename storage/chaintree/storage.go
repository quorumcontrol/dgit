package chaintree

import (
	"github.com/quorumcontrol/decentragit-remote/storage/split"
	"gopkg.in/src-d/go-git.v4/storage"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

func NewStorage(config *StorageConfig) storage.Storer {
	return split.NewStorage(&split.StorageMap{
		ObjectStorage:    NewObjectStorage(config), // TODO: Make object storage configurable (chaintree / sia) from data in chaintree
		ShallowStorage:   memory.NewStorage(),
		ReferenceStorage: NewReferenceStorage(config),
		IndexStorage:     memory.NewStorage(),
		ConfigStorage:    memory.NewStorage(),
	})
}
