package chaintree

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	"github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/objfile"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

var log = logging.Logger("dgit.storage.chaintree")

type ObjectStorage struct {
	storer.EncodedObjectStorer
	ctx       context.Context
	tupelo    *client.Client
	chainTree *consensus.SignedChainTree
	treeKey   *ecdsa.PrivateKey
	log       *zap.SugaredLogger
}

func NewObjectStorage(ctx context.Context, tupelo *client.Client, chainTree *consensus.SignedChainTree, treeKey *ecdsa.PrivateKey) storer.EncodedObjectStorer {
	did := chainTree.MustId()
	return &ObjectStorage{
		ctx:       ctx,
		tupelo:    tupelo,
		chainTree: chainTree,
		treeKey:   treeKey,
		log:       log.Named(did[len(did)-6:]),
	}
}

func (s *ObjectStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

func (s *ObjectStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	s.log.Debugf("saving %s with type %s", o.Hash().String(), o.Type().String())

	if s.treeKey == nil {
		return plumbing.ZeroHash, fmt.Errorf("Must specify treeKey during NewObjectStorage init")
	}

	if o.Type() == plumbing.OFSDeltaObject || o.Type() == plumbing.REFDeltaObject {
		return plumbing.ZeroHash, plumbing.ErrInvalidType
	}

	buf := bytes.NewBuffer(nil)

	writer := objfile.NewWriter(buf)
	defer writer.Close()

	reader, err := o.Reader()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer reader.Close()

	if err := writer.WriteHeader(o.Type(), o.Size()); err != nil {
		return plumbing.ZeroHash, err
	}

	if _, err = io.Copy(writer, reader); err != nil {
		return plumbing.ZeroHash, err
	}
	writer.Close()

	objectBytes, err := ioutil.ReadAll(buf)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// TODO: batch these
	// TODO: save each git object as cid
	//   currently objects/sha1[0:2]/ is a map with { sha1[2:] => cbor bytes }
	//   should be objects/sha1[0:2]/ is a map with { sha1[2:] => cid }
	transaction, err := chaintree.NewSetDataTransaction(objectWritePath(o.Hash()), objectBytes)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = s.tupelo.PlayTransactions(s.ctx, s.chainTree, s.treeKey, []*transactions.Transaction{transaction})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return o.Hash(), nil
}

func (s *ObjectStorage) HasEncodedObject(h plumbing.Hash) (err error) {
	if _, err := s.EncodedObject(plumbing.AnyObject, h); err != nil {
		return err
	}
	return nil
}

func (s *ObjectStorage) EncodedObjectSize(h plumbing.Hash) (size int64, err error) {
	o, err := s.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return 0, err
	}
	return o.Size(), nil
}

func (s *ObjectStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	s.log.Debugf("fetching %s with type %s", h.String(), t.String())

	valUncast, _, err := s.chainTree.ChainTree.Dag.Resolve(s.ctx, objectReadPath(h))
	if err == format.ErrNotFound {
		s.log.Debugf("%s not found", h.String())
		return nil, plumbing.ErrObjectNotFound
	}
	if err != nil {
		s.log.Errorf("chaintree resolve error for %s: %v", h.String(), err)
		return nil, err
	}
	if valUncast == nil {
		s.log.Debugf("%s not found", h.String())
		return nil, plumbing.ErrObjectNotFound
	}

	o := &plumbing.MemoryObject{}

	buf := bytes.NewBuffer(valUncast.([]byte))
	reader, err := objfile.NewReader(buf)
	if err != nil {
		s.log.Errorf("new reader error for %s: %v", h.String(), err)
		return nil, err
	}
	defer reader.Close()

	objType, size, err := reader.Header()
	if err != nil {
		s.log.Errorf("error decoding header for %s: %v", h.String(), err)
		return nil, err
	}

	o.SetType(objType)
	o.SetSize(size)

	if plumbing.AnyObject != t && o.Type() != t {
		s.log.Debugf("%s not found, mismatched types, expected %s, got %s", h.String(), t.String(), o.Type().String())
		return nil, plumbing.ErrObjectNotFound
	}

	if _, err = io.Copy(o, reader); err != nil {
		s.log.Errorf("error filling object %s: %v", h.String(), err)
		return nil, err
	}

	return o, nil
}

func (s *ObjectStorage) IterEncodedObjects(t plumbing.ObjectType) (storer.EncodedObjectIter, error) {
	return &EncodedObjectIter{
		store: s,
	}, nil
}

type EncodedObjectIter struct {
	storer.EncodedObjectIter
	store *ObjectStorage
}

// Next returns the next object from the iterator. If the iterator has reached
// the end it will return io.EOF as an error. If the object is retreieved
// successfully error will be nil.
func (iter *EncodedObjectIter) Next() (plumbing.EncodedObject, error) {
	// var recursiveFetch func(pathSlice []string) error

	// recursiveFetch = func(pathSlice []string) error {
	// 	valUncast, _, err := chainTree.ChainTree.Dag.Resolve(context.Background(), pathSlice)

	// 	if err != nil {
	// 		return err
	// 	}

	// 	switch val := valUncast.(type) {
	// 	case map[string]interface{}:
	// 		for key := range val {
	// 			recursiveFetch(append(pathSlice, key))
	// 		}
	// 	case string:
	// 		refName := plumbing.ReferenceName(strings.Join(pathSlice[2:], "/"))
	// 		log.Debugf("ref name is: %s", refName)
	// 		log.Debugf("val is: %s", val)
	// 		ref := plumbing.NewHashReference(refName, plumbing.NewHash(val))

	// 		err = ar.AddReference(ref)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// 	return nil
	// }

	// err = recursiveFetch([]string{"tree", "data", "objects"})

	return nil, nil
}

func (iter *EncodedObjectIter) ForEach(cb func(plumbing.EncodedObject) error) error {
	return storer.ForEachIterator(iter, cb)
}

func (iter *EncodedObjectIter) Close() {
}

func objectReadPath(h plumbing.Hash) []string {
	prefix := h.String()[0:2]
	key := h.String()[2:]
	return []string{"tree", "data", "objects", prefix, key}
}

func objectWritePath(h plumbing.Hash) string {
	return strings.Join(objectReadPath(h)[2:], "/")
}
