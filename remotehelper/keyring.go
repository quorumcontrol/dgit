package remotehelper

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/99designs/keyring"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var secureKeyringBackends = []keyring.BackendType{
	keyring.WinCredBackend,
	keyring.KeychainBackend,
	keyring.SecretServiceBackend,
	keyring.KWalletBackend,
	keyring.PassBackend,
}

func NewDefaultKeyring() (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName:                    "dgit",
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		AllowedBackends:                secureKeyringBackends,
	})
}

func GetPrivateKey(kr keyring.Keyring) (key *ecdsa.PrivateKey, isNew bool, err error) {
	// TODO: scope keyring by usernames
	keyName := "default"

	privateKeyItem, err := kr.Get(keyName)
	if err == keyring.ErrKeyNotFound {
		isNew = true

		privateKey, err := crypto.GenerateKey()
		if err != nil {
			return nil, isNew, err
		}

		privateKeyItem = keyring.Item{
			Key:   keyName,
			Label: "dgit." + keyName,
			Data:  []byte(hexutil.Encode(crypto.FromECDSA(privateKey))),
		}

		err = kr.Set(privateKeyItem)
		if err != nil {
			return nil, isNew, fmt.Errorf("error saving private key for dgit: %v", err)
		}
	}

	privateKeyBytes, err := hexutil.Decode(string(privateKeyItem.Data))
	if err != nil {
		return nil, isNew, fmt.Errorf("error decoding user private key: %v", err)
	}

	key, err = crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return key, isNew, fmt.Errorf("couldn't unmarshal ECDSA private key: %v", err)
	}

	return key, isNew, nil
}
