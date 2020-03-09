package dgit

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

const PrivateKeyAuthName = "private-key-auth"

type PrivateKeyAuth struct {
	transport.AuthMethod
	key *ecdsa.PrivateKey
}

func NewPrivateKeyAuth(key *ecdsa.PrivateKey) *PrivateKeyAuth {
	return &PrivateKeyAuth{key: key}
}

func (a *PrivateKeyAuth) Name() string {
	return PrivateKeyAuthName
}

func (a *PrivateKeyAuth) Key() *ecdsa.PrivateKey {
	return a.key
}

func (a *PrivateKeyAuth) String() string {
	return crypto.PubkeyToAddress(a.key.PublicKey).String()
}
