package chaintree

import (
	"context"
	"sort"
	"strings"

	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	gitstorage "gopkg.in/src-d/go-git.v4/storage"

	"github.com/quorumcontrol/dgit/storage"
)

type ReferenceStorage struct {
	*storage.Config
	log *zap.SugaredLogger
}

var _ storer.ReferenceStorer = (*ReferenceStorage)(nil)

func NewReferenceStorage(config *storage.Config) storer.ReferenceStorer {
	did := config.ChainTree.MustId()
	return &ReferenceStorage{
		config,
		log.Named(did[len(did)-6:]),
	}
}

func (s *ReferenceStorage) SetReference(ref *plumbing.Reference) error {
	log.Debugf("set reference %s to %s", ref.Name().String(), ref.Hash().String())
	return s.setData(ref.Name().String(), ref.Hash().String())
}

func (s *ReferenceStorage) setData(key string, val interface{}) error {
	txn, err := chaintree.NewSetDataTransaction(key, val)
	if err != nil {
		return err
	}

	_, err = s.Tupelo.PlayTransactions(s.Ctx, s.ChainTree, s.PrivateKey, []*transactions.Transaction{txn})
	if err != nil {
		return err
	}

	return nil
}

func (s *ReferenceStorage) CheckAndSetReference(ref *plumbing.Reference, old *plumbing.Reference) error {
	if ref == nil {
		return nil
	}

	if old != nil {
		tmp, err := s.Reference(ref.Name())

		if err != nil && err != plumbing.ErrReferenceNotFound {
			return err
		}

		if tmp != nil && tmp.Hash() != old.Hash() {
			return gitstorage.ErrReferenceHasChanged
		}
	}

	return s.SetReference(ref)
}

// Reference returns the reference for a given reference name.
func (s *ReferenceStorage) Reference(n plumbing.ReferenceName) (*plumbing.Reference, error) {
	refPath := append([]string{"tree", "data"}, strings.Split(n.String(), "/")...)
	valUncast, _, err := s.ChainTree.ChainTree.Dag.Resolve(context.Background(), refPath)
	if err != nil {
		return nil, err
	}

	if valStr, ok := valUncast.(string); ok {
		return plumbing.NewHashReference(n, plumbing.NewHash(valStr)), nil
	}

	return nil, plumbing.ErrReferenceNotFound
}

func (s *ReferenceStorage) RemoveReference(n plumbing.ReferenceName) error {
	return s.setData(n.String(), nil)
}

func (s *ReferenceStorage) CountLooseRefs() (int, error) {
	allRefs, err := s.references()
	if err != nil {
		return 0, err
	}
	return len(allRefs), nil
}

func (r *ReferenceStorage) PackRefs() error {
	return nil
}

func (s *ReferenceStorage) references() ([]*plumbing.Reference, error) {
	refs := []*plumbing.Reference{}

	var recursiveFetch func(pathSlice []string) error

	recursiveFetch = func(pathSlice []string) error {
		log.Debugf("fetching references under: %s", pathSlice)

		valUncast, _, err := s.ChainTree.ChainTree.Dag.Resolve(context.Background(), pathSlice)

		if err != nil {
			return err
		}

		switch val := valUncast.(type) {
		case map[string]interface{}:
			sortedKeys := make([]string, len(val))
			i := 0
			for key := range val {
				sortedKeys[i] = key
				i++
			}
			sort.Strings(sortedKeys)

			for _, key := range sortedKeys {
				recursiveFetch(append(pathSlice, key))
			}
		case string:
			refName := plumbing.ReferenceName(strings.Join(pathSlice[2:], "/"))
			log.Debugf("ref name is: %s", refName)
			log.Debugf("val is: %s", val)
			refs = append(refs, plumbing.NewHashReference(refName, plumbing.NewHash(val)))
		}
		return nil
	}

	err := recursiveFetch([]string{"tree", "data", "refs"})
	if err != nil {
		return nil, err
	}

	return refs, nil
}

func (s *ReferenceStorage) IterReferences() (storer.ReferenceIter, error) {
	allRefs, err := s.references()
	if err != nil {
		return nil, err
	}
	return storer.NewReferenceSliceIter(allRefs), nil
}
