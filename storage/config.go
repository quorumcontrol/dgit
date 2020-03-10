package storage

import (
	"context"
	"crypto/ecdsa"

	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
)

type Config struct {
	Ctx        context.Context
	Tupelo     *tupelo.Client
	ChainTree  *consensus.SignedChainTree
	PrivateKey *ecdsa.PrivateKey
}
