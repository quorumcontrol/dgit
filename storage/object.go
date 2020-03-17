package storage

import (
	"strings"

	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type ChaintreeObjectStorer interface {
	storer.EncodedObjectStorer
	Chaintree() *chaintree.ChainTree
}

type ChaintreeObjectStorage struct {
	*Config
}

var ObjectsBasePath = []string{"tree", "data", "objects"}

func ObjectReadPath(h plumbing.Hash) []string {
	prefix := h.String()[0:2]
	key := h.String()[2:]
	return append(ObjectsBasePath, prefix, key)
}

func ObjectWritePath(h plumbing.Hash) string {
	return strings.Join(ObjectReadPath(h)[2:], "/")
}

func (s *ChaintreeObjectStorage) Chaintree() *chaintree.ChainTree {
	return s.ChainTree.ChainTree
}

func (s *ChaintreeObjectStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}
