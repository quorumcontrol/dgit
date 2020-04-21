package chaintree

import (
	"context"
	"fmt"
	"io"

	"github.com/go-git/go-git/v5/config"
	"github.com/quorumcontrol/chaintree/dag"

	"github.com/quorumcontrol/dgit/storage"
	"github.com/quorumcontrol/dgit/storage/siaskynet"

	"github.com/go-git/go-git/v5/plumbing/storer"
	gitstorage "github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

var RepoConfigPath = []string{"tree", "data", "config"}

const defaultStorageProvider = "chaintree"

type ChaintreeStorage struct {
	storer.EncodedObjectStorer
	storer.ReferenceStorer
	storer.ShallowStorer
	storer.IndexStorer
	config.ConfigStorer
}

func NewStorage(config *storage.Config) (gitstorage.Storer, error) {
	ctx := context.Background()

	objStorageProvider, err := getObjectStorageProvider(ctx, config.ChainTree.ChainTree.Dag)
	if err != nil {
		return nil, err
	}

	var objStorage storer.EncodedObjectStorer

	switch objStorageProvider {
	case "chaintree":
		objStorage = NewObjectStorage(config)
	case "siaskynet":
		objStorage = siaskynet.NewObjectStorage(config)
	default:
		return nil, fmt.Errorf("unknown object storage type: %s", objStorageProvider)
	}

	return &ChaintreeStorage{
		objStorage,
		NewReferenceStorage(config),
		memory.NewStorage(),
		memory.NewStorage(),
		memory.NewStorage(),
	}, nil
}

func (s *ChaintreeStorage) Module(_ string) (gitstorage.Storer, error) {
	return nil, fmt.Errorf("ChaintreeStorage.Module not implemented")
}

func getObjectStorageProvider(ctx context.Context, dag *dag.Dag) (string, error) {
	configUncast, _, err := dag.Resolve(ctx, RepoConfigPath)
	if err != nil {
		return "", fmt.Errorf("could not resolve repo config in chaintree: %w", err)
	}
	// repo hasn't been configured yet
	if configUncast == nil {
		return defaultStorageProvider, nil
	}

	var (
		ctConfig map[string]interface{}
		ok       bool
	)
	if ctConfig, ok = configUncast.(map[string]interface{}); !ok {
		return "", fmt.Errorf("could not cast config to map[string]interface{}: was %T instead", configUncast)
	}

	objectStorageConfigUncast := ctConfig["objectStorage"]
	var objectStorageConfig map[string]interface{}
	if objectStorageConfig, ok = objectStorageConfigUncast.(map[string]interface{}); !ok {
		return "", fmt.Errorf("could not cast objectStorage config to map[string]interface{}: was %T instead", objectStorageConfigUncast)
	}

	objStorageType, ok := objectStorageConfig["type"].(string)
	if !ok {
		return "", fmt.Errorf("could not cast objectStorage config type to string; was %T instead", objectStorageConfig["type"])
	}

	if objStorageType == "" {
		return defaultStorageProvider, nil
	}

	return objStorageType, nil
}

func (s *ChaintreeStorage) PackfileWriter() (io.WriteCloser, error) {
	pw, ok := s.EncodedObjectStorer.(storer.PackfileWriter)
	if !ok {
		return nil, fmt.Errorf("could not cast object storer to packfile writer")
	}

	return pw.PackfileWriter()
}
