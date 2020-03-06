package chaintree

import (
	"context"
	"crypto/ecdsa"

	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
)

type StorageConfig struct {
	Ctx        context.Context
	Tupelo     *tupelo.Client
	ChainTree  *consensus.SignedChainTree
	PrivateKey *ecdsa.PrivateKey
}
