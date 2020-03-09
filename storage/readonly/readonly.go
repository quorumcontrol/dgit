package readonly

import (
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/src-d/go-git.v4/storage/transactional"
)

type store struct {
	storer.EncodedObjectStorer
	storer.ReferenceStorer
	storer.ShallowStorer
	storer.IndexStorer
	config.ConfigStorer
	base storage.Storer
}

// readonly store is a shortcut to transactional store without
// the commit hooks, meaning any write changes performed to this store
// are never persisted and only held in memory throughout the duration
// of this object
func NewStorage(base storage.Storer) storage.Storer {
	return &store{
		NewObjectStorage(base),
		NewReferenceStorage(base),
		NewShallowStorage(base),
		NewIndexStorage(base),
		NewConfigStorage(base),
		base,
	}
}

func (s *store) Module(name string) (storage.Storer, error) {
	modStore, err := s.base.Module(name)
	if err != nil {
		return nil, err
	}
	return NewStorage(modStore), nil
}

func NewConfigStorage(base storage.Storer) config.ConfigStorer {
	return transactional.NewConfigStorage(base, memory.NewStorage())
}

func NewShallowStorage(base storage.Storer) storer.ShallowStorer {
	return transactional.NewShallowStorage(base, memory.NewStorage())
}

func NewIndexStorage(base storage.Storer) storer.IndexStorer {
	return transactional.NewIndexStorage(base, memory.NewStorage())
}

func NewReferenceStorage(base storage.Storer) storer.ReferenceStorer {
	return transactional.NewReferenceStorage(base, memory.NewStorage())
}

func NewObjectStorage(base storage.Storer) storer.EncodedObjectStorer {
	return transactional.NewObjectStorage(base, memory.NewStorage())
}
