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

func migrateOldDefaultKey(kr Keyring, keyName string) (*keyringlib.Item, error) {
	oldDefault, err := kr.Get("default")
	if err == keyringlib.ErrKeyNotFound {
		log.Debugf("no dgit.default key found")
		return nil, ErrKeyNotFound
	}

	if err == nil {
		log.Debugf("migrating old dgit.default key to dgit.%s", keyName)
		oldDefault.Key = keyName
		oldDefault.Label = "dgit." + keyName
		err = kr.Set(oldDefault)
		if err != nil {
			log.Errorf("error migrating dgit.default key: %v", err)
			return nil, err
		}
		remErr := kr.Remove("default")
		if remErr != nil {
			log.Warnf("error removing old dgit.default key: %v", remErr)
		}
	}

	return &oldDefault, err
}

func FindPrivateKey(kr Keyring, keyName string) (key *ecdsa.PrivateKey, err error) {
	log.Debugf("finding private key %s", keyName)
	privateKeyItem, err := kr.Get(keyName)
	if err == keyringlib.ErrKeyNotFound {
		log.Debugf("private key %s not found; attempting to migrate old dgit.default key", keyName)
		migratedItem, err := migrateOldDefaultKey(kr, keyName)
		if err != nil {
			return nil, err
		}
		privateKeyItem = *migratedItem
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

func FindOrCreatePrivateKey(kr Keyring, keyName string) (key *ecdsa.PrivateKey, isNew bool, err error) {
	privateKey, err := FindPrivateKey(kr, keyName)
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
