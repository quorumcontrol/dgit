package keyring

import (
	"crypto/ecdsa"
	"fmt"

	keyringlib "github.com/99designs/keyring"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("dgit.keyring")

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

// TODO: scope private key by usernames
var keyName = "default"

var ErrKeyNotFound = keyringlib.ErrKeyNotFound

func NewDefault() (Keyring, error) {
	kr, err := keyringlib.Open(keyringlib.Config{
		ServiceName:                    "dgit",
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		AllowedBackends:                secureKeyringBackends,
	})
	log.Info("keyring provider: " + Name(kr))
	return kr, err
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

func FindPrivateKey(kr Keyring) (key *ecdsa.PrivateKey, err error) {
	privateKeyItem, err := kr.Get(keyName)
	if err == keyringlib.ErrKeyNotFound {
		return nil, ErrKeyNotFound
	}

	privateKeyBytes, err := hexutil.Decode(string(privateKeyItem.Data))
	if err != nil {
		return nil, fmt.Errorf("error decoding user private key: %v", err)
	}

	key, err = crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal ECDSA private key: %v", err)
	}

	return key, nil
}

func FindOrCreatePrivateKey(kr Keyring) (key *ecdsa.PrivateKey, isNew bool, err error) {
	privateKey, err := FindPrivateKey(kr)
	if err == nil {
		return privateKey, false, nil
	}

	if err != ErrKeyNotFound {
		return nil, false, err
	}

	privateKey, err = crypto.GenerateKey()
	if err != nil {
		return nil, true, err
	}

	privateKeyItem := keyringlib.Item{
		Key:   keyName,
		Label: "dgit." + keyName,
		Data:  []byte(hexutil.Encode(crypto.FromECDSA(privateKey))),
	}

	err = kr.Set(privateKeyItem)
	if err != nil {
		return nil, true, fmt.Errorf("error saving private key for dgit: %v", err)
	}

	return privateKey, true, nil
}
