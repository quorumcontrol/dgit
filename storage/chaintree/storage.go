package chaintree

import (
	"context"
	"fmt"

	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/quorumcontrol/decentragit-remote/storage"
	"github.com/quorumcontrol/decentragit-remote/storage/siaskynet"
	"github.com/quorumcontrol/decentragit-remote/storage/split"

	gitstorage "gopkg.in/src-d/go-git.v4/storage"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var RepoConfigPath = []string{"tree", "data", "dgit", "config"}

func NewStorage(config *storage.Config) (gitstorage.Storer, error) {
	ctx := context.Background()

	ct := config.ChainTree

	configUncast, remaining, err := ct.ChainTree.Dag.Resolve(ctx, RepoConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve repo config in chaintree: %w", err)
	}
	if len(remaining) > 0 {
		return nil, fmt.Errorf("path elements remaining when trying to resolve repo config: %v", remaining)
	}

	var (
		ctConfig map[string]interface{}
		ok bool
	)
	if ctConfig, ok = configUncast.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("could not cast config to map[string]interface{}: was %T instead", configUncast)
	}

	objectStorageConfigUncast := ctConfig["objectStorage"]
	var objectStorageConfig map[string]string
	if objectStorageConfig, ok = objectStorageConfigUncast.(map[string]string); !ok {
		return nil, fmt.Errorf("could not cast objectStorage config to map[string]string: was %T instead", objectStorageConfigUncast)
	}

	var objStorage storer.EncodedObjectStorer

	switch t := objectStorageConfig["type"]; t {
	case "chaintree", "":
		objStorage = NewObjectStorage(config)
	case "siaskynet":
		objStorage = siaskynet.NewObjectStorage(config)
	default:
		return nil, fmt.Errorf("unknown object storage type: %s", t)
	}

	return split.NewStorage(&split.StorageMap{
		ObjectStorage:    objStorage,
		ShallowStorage:   memory.NewStorage(),
		ReferenceStorage: NewReferenceStorage(config),
		IndexStorage:     memory.NewStorage(),
		ConfigStorage:    memory.NewStorage(),
	}), nil
}
