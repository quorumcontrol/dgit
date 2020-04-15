package keyring

import (
	"crypto/ecdsa"
	"fmt"

	keyringlib "github.com/99designs/keyring"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("dgit.keyring")

type Keyring struct {
	kr keyringlib.Keyring
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

func NewDefault() (*Keyring, error) {
	kr, err := keyringlib.Open(keyringlib.Config{
		ServiceName:                    "dgit",
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		AllowedBackends:                secureKeyringBackends,
	})
	if err != nil {
		return nil, err
	}
	k := &Keyring{kr}
	log.Info("keyring provider: " + k.Name())
	return k, nil
}

func NewMemory() *Keyring {
	return &Keyring{keyringlib.NewArrayKeyring([]keyringlib.Item{})}
}

func (k *Keyring) Name() string {
	typeName := fmt.Sprintf("%T", k.kr)
	name, ok := KeyringPrettyNames[typeName]
	if !ok {
		return typeName
	}
	return name
}

func (k *Keyring) migrateOldDefaultKey(keyName string) (*keyringlib.Item, error) {
	oldDefault, err := k.kr.Get("default")
	if err == keyringlib.ErrKeyNotFound {
		log.Debugf("no dgit.default key found")
		return nil, ErrKeyNotFound
	}

	if err == nil {
		log.Debugf("migrating old dgit.default key to dgit.%s", keyName)
		oldDefault.Key = keyName
		oldDefault.Label = "dgit." + keyName
		err = k.kr.Set(oldDefault)
		if err != nil {
			log.Errorf("error migrating dgit.default key: %v", err)
			return nil, err
		}
		remErr := k.kr.Remove("default")
		if remErr != nil {
			log.Warnf("error removing old dgit.default key: %v", remErr)
		}
	}

	return &oldDefault, err
}

func (k *Keyring) FindPrivateKey(keyName string) (key *ecdsa.PrivateKey, err error) {
	log.Debugf("finding private key %s", keyName)
	privateKeyItem, err := k.kr.Get(keyName)
	if err == keyringlib.ErrKeyNotFound {
		log.Debugf("private key %s not found; attempting to migrate old dgit.default key", keyName)
		migratedItem, err := k.migrateOldDefaultKey(keyName)
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

func (k *Keyring) CreatePrivateKey(keyName string, seed []byte) (*ecdsa.PrivateKey, error) {
	derivedKeyPaths, err := accounts.ParseDerivationPath("m/44'/1392825'/0'/0")
	if err != nil {
		return nil, err
	}

	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.Params{
		HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4},
	})
	if err != nil {
		return nil, err
	}

	key := masterKey
	for _, n := range derivedKeyPaths {
		key, err = key.Child(n)
		if err != nil {
			panic(err)
		}

	}

	ecPrivateKey, err := key.ECPrivKey()
	privateKey := ecPrivateKey.ToECDSA()

	privateKeyItem := keyringlib.Item{
		Key:   keyName,
		Label: "dgit." + keyName,
		Data:  []byte(hexutil.Encode(crypto.FromECDSA(privateKey))),
	}

	err = k.kr.Set(privateKeyItem)
	if err != nil {
		return nil, fmt.Errorf("error saving private key for dgit: %v", err)
	}

	return privateKey, nil
}

func (k *Keyring) DeletePrivateKey(keyName string) {
	err := k.kr.Remove(keyName)
	if err != nil {
		log.Warnf("error removing dgit.%s key: %w", keyName, err)
	}
}
