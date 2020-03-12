package keyring

import (
	"crypto/ecdsa"
	"fmt"

	keyringlib "github.com/99designs/keyring"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Keyring interface {
	keyringlib.Keyring
}

var secureKeyringBackends = []keyringlib.BackendType{
	keyringlib.WinCredBackend,
	keyringlib.KeychainBackend,
	keyringlib.SecretServiceBackend,
	keyringlib.KWalletBackend,
	keyringlib.PassBackend,
}

var KeyringPrettyNames = map[string]string{
	"*keyring.keychain":       "macOS Keychain Access",
	"*keyring.kwalletKeyring": "KWallet (KDE Wallet Manager)",
	"*keyring.windowsKeyring": "Windows Credential Manager",
	"*keyring.secretsKeyring": "libsecret",
	"*keyring.passKeyring":    "pass",
}

func NewDefault() (Keyring, error) {
	return keyringlib.Open(keyringlib.Config{
		ServiceName:                    "dgit",
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		AllowedBackends:                secureKeyringBackends,
	})
}

func NewMemory() Keyring {
	return keyringlib.NewArrayKeyring([]keyringlib.Item{})
}

func Name(kr Keyring) string {
	typeName := fmt.Sprintf("%T", kr)
	name, ok := KeyringPrettyNames[typeName]
	if !ok {
		return typeName
	}
	return name
}

func GetPrivateKey(kr Keyring) (key *ecdsa.PrivateKey, isNew bool, err error) {
	// TODO: scope keyring by usernames
	keyName := "default"

	privateKeyItem, err := kr.Get(keyName)
	if err == keyringlib.ErrKeyNotFound {
		isNew = true

		privateKey, err := crypto.GenerateKey()
		if err != nil {
			return nil, isNew, err
		}

		privateKeyItem = keyringlib.Item{
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
