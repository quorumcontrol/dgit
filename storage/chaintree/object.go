package chaintree

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"

	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"go.uber.org/zap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/objfile"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

var log = logging.Logger("dgit.storage.chaintree")

type ObjectStorage struct {
	*StorageConfig
	log *zap.SugaredLogger
}

var _ storer.EncodedObjectStorer = (*ObjectStorage)(nil)

func NewObjectStorage(config *StorageConfig) storer.EncodedObjectStorer {
	did := config.ChainTree.MustId()
	return &ObjectStorage{
		config,
		log.Named(did[len(did)-6:]),
	}
}

func (s *ObjectStorage) NewEncodedObject() plumbing.EncodedObject {
	return &plumbing.MemoryObject{}
}

// TODO: implement PackfileWriter() for batch SetEncodedObject

func (s *ObjectStorage) SetEncodedObject(o plumbing.EncodedObject) (plumbing.Hash, error) {
	s.log.Debugf("saving %s with type %s", o.Hash().String(), o.Type().String())

	if s.PrivateKey == nil {
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

	// TODO: batch these, see PackfileWriter()
	// TODO: save each git object as cid
	//   currently objects/sha1[0:2]/ is a map with { sha1[2:] => cbor bytes }
	//   should be objects/sha1[0:2]/ is a map with { sha1[2:] => cid }
	transaction, err := chaintree.NewSetDataTransaction(objectWritePath(o.Hash()), objectBytes)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = s.Tupelo.PlayTransactions(s.Ctx, s.ChainTree, s.PrivateKey, []*transactions.Transaction{transaction})
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

	valUncast, _, err := s.ChainTree.ChainTree.Dag.Resolve(s.Ctx, objectReadPath(h))
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
		t:     t,
	}, nil
}

type EncodedObjectIter struct {
	storer.EncodedObjectIter
	t                    plumbing.ObjectType
	store                *ObjectStorage
	shards               []string
	currentShardIndex    int
	currentShardKeys     []string
	currentShardKeyIndex int
}

func (iter *EncodedObjectIter) getLeafKeysSorted(path []string) ([]string, error) {
	valUncast, _, err := iter.store.ChainTree.ChainTree.Dag.Resolve(context.Background(), path)
	if err != nil {
		return nil, err
	}
	if valUncast == nil {
		return nil, io.EOF
	}

	keys := make([]string, len(valUncast.(map[string]interface{})))
	i := 0
	for key := range valUncast.(map[string]interface{}) {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys, nil
}

func (iter *EncodedObjectIter) getShards() ([]string, error) {
	return iter.getLeafKeysSorted(objectsBasePath)
}

func (iter *EncodedObjectIter) getShardKeys(shard string) ([]string, error) {
	return iter.getLeafKeysSorted(append(objectsBasePath, shard))
}

// Next returns the next object from the iterator. If the iterator has reached
// the end it will return io.EOF as an error. If the object is retreieved
// successfully error will be nil.
func (iter *EncodedObjectIter) Next() (plumbing.EncodedObject, error) {
	if len(iter.shards) == 0 {
		shards, err := iter.getShards()
		if err != nil {
			return nil, err
		}
		if len(shards) == 0 {
			return nil, io.EOF
		}
		iter.shards = shards
		iter.currentShardIndex = 0
	}

	if (iter.currentShardIndex + 1) > len(iter.shards) {
		return nil, io.EOF
	}

	currentShard := iter.shards[iter.currentShardIndex]
	if len(iter.currentShardKeys) == 0 {
		shardKeys, err := iter.getShardKeys(currentShard)
		if err != nil {
			return nil, err
		}
		if len(shardKeys) == 0 {
			return nil, io.EOF
		}
		iter.currentShardKeys = shardKeys
		iter.currentShardKeyIndex = 0
	}

	currentObjectHash := plumbing.NewHash(currentShard + iter.currentShardKeys[iter.currentShardKeyIndex])

	fmt.Printf("Fetching %s of type %s\n", currentObjectHash.String(), iter.t.String())

	iter.currentShardKeyIndex++

	// if the last key in the shard, empty out shard keys and increment shard num to trigger fetching next shard
	if (iter.currentShardKeyIndex + 1) > len(iter.currentShardKeys) {
		iter.currentShardKeys = []string{}
		iter.currentShardKeyIndex = 0
		iter.currentShardIndex++
	}

	obj, err := iter.store.EncodedObject(plumbing.AnyObject, currentObjectHash)
	if err != nil {
		return nil, err
	}

	// object was not the type being searched for, move to the next object
	if plumbing.AnyObject != iter.t && obj.Type() != iter.t {
		return iter.Next()
	}

	return obj, nil
}

func (iter *EncodedObjectIter) ForEach(cb func(plumbing.EncodedObject) error) error {
	return storer.ForEachIterator(iter, cb)
}

func (iter *EncodedObjectIter) Close() {
}

var objectsBasePath = []string{"tree", "data", "objects"}

func objectReadPath(h plumbing.Hash) []string {
	prefix := h.String()[0:2]
	key := h.String()[2:]
	return append(objectsBasePath, prefix, key)
}

func objectWritePath(h plumbing.Hash) string {
	return strings.Join(objectReadPath(h)[2:], "/")
}
