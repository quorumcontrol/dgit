package storage

import (
	"context"
	"crypto/ecdsa"

	"github.com/quorumcontrol/tupelo/sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"
)

type Config struct {
	Ctx        context.Context
	Tupelo     *tupelo.Client
	ChainTree  *consensus.SignedChainTree
	PrivateKey *ecdsa.PrivateKey
}
