package storage

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type EncodedObjectIter struct {
	storer.EncodedObjectIter
	t                    plumbing.ObjectType
	store                ChaintreeObjectStorer
	shards               []string
	currentShardIndex    int
	currentShardKeys     []string
	currentShardKeyIndex int
}

func NewEncodedObjectIter(store ChaintreeObjectStorer, t plumbing.ObjectType) *EncodedObjectIter {
	return &EncodedObjectIter{
		store: store,
		t:     t,
	}
}

func (iter *EncodedObjectIter) getLeafKeysSorted(path []string) ([]string, error) {
	valUncast, _, err := iter.store.Chaintree().Dag.Resolve(context.Background(), path)
	if err != nil {
		return nil, err
	}
	if valUncast == nil {
		return nil, io.EOF
	}

	valMapUncast, ok := valUncast.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("path %v is %T, expected map", path, valUncast)
	}

	keys := make([]string, len(valMapUncast))
	i := 0
	for key := range valMapUncast {
		keys[i] = key
		i++
	}
	sort.Strings(keys)
	return keys, nil
}

func (iter *EncodedObjectIter) getShards() ([]string, error) {
	return iter.getLeafKeysSorted(ObjectsBasePath)
}

func (iter *EncodedObjectIter) getShardKeys(shard string) ([]string, error) {
	return iter.getLeafKeysSorted(append(ObjectsBasePath, shard))
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

	if iter.currentShardIndex >= len(iter.shards) {
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

	iter.currentShardKeyIndex++

	// if the last key in the shard, empty out shard keys and increment shard num to trigger fetching next shard
	if iter.currentShardKeyIndex >= len(iter.currentShardKeys) {
		iter.currentShardKeys = []string{}
		iter.currentShardKeyIndex = 0
		iter.currentShardIndex++
	}

	obj, err := iter.store.EncodedObject(plumbing.AnyObject, currentObjectHash)
	if err != nil {
		return nil, err
	}

	// if object was not the type being searched for, move to the next object
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
