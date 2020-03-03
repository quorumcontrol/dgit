package split

import (
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
)

type store struct {
	storer.EncodedObjectStorer
	storer.ReferenceStorer
	storer.ShallowStorer
	storer.IndexStorer
	config.ConfigStorer
	opts *StorageMap
}

type StorageMap struct {
	ObjectStorage    storer.EncodedObjectStorer
	ReferenceStorage storer.ReferenceStorer
	ShallowStorage   storer.ShallowStorer
	IndexStorage     storer.IndexStorer
	ConfigStorage    config.ConfigStorer
}

func NewStorage(opts *StorageMap) storage.Storer {
	return &store{
		opts.ObjectStorage,
		opts.ReferenceStorage,
		opts.ShallowStorage,
		opts.IndexStorage,
		opts.ConfigStorage,
		opts,
	}
}

func (s *store) Module(name string) (storage.Storer, error) {
	return NewStorage(s.opts), nil
}
