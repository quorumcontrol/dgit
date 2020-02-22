package repo

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-cid"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/decentragit-remote/constants"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	"github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"github.com/quorumcontrol/tupelo-go-sdk/p2p"
)

const userPrivateKey = "0x5643765d2b05b1b8fc6d5419c88aa0a5be8ef410e8ebc9f4e0ce752334ff2f33"

type Repo struct {
	ctx        context.Context
	remoteName string
	url        string
	protocol   string
	privateKey *ecdsa.PrivateKey
	tupelo     *client.Client
	bitswap    *p2p.BitswapPeer
	chaintree  *consensus.SignedChainTree
}

// TODO: inject tupelo host instead, maybe spawn long lived process with a host running too?
func New(ctx context.Context, remoteName string, url string) (*Repo, error) {
	// TODO: move to secure storage, maybe keyring again?
	privateKeyBytes, err := hexutil.Decode(userPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding user private key: %v", err)
	}

	ecdsaPrivate, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal ECDSA private key: %v", err)
	}

	r := &Repo{
		remoteName: remoteName,
		url:        url,
		protocol:   constants.Protocol,
		privateKey: ecdsaPrivate,
		ctx:        ctx,
	}

	return r, r.Initialize()
}

func (r *Repo) RemoteName() string {
	return r.remoteName
}

func (r *Repo) Url() string {
	return r.url
}

func (r *Repo) Dir() string {
	return path.Join(os.Getenv("GIT_DIR"), r.protocol, r.RemoteName())
}

// refs/heads/fix/move-snowball-ticker
func (r *Repo) HeadRefs() string {
	return fmt.Sprintf("refs/heads/*:refs/%s/%s/*", r.protocol, r.RemoteName())
}

// func (r *Repo) BranchRefs() string {
// 	return fmt.Sprintf("refs/heads/branches/*:refs/%s/%s/branches/*", r.protocol, r.RemoteName())
// }

// refs/tags/v0.0.2
func (r *Repo) TagRefs() string {
	return fmt.Sprintf("refs/heads/tags/*:refs/%s/%s/tags/*", r.protocol, r.RemoteName())
}

// refs/pull/107/head

func (r *Repo) Capabilities() []string {
	return []string{
		"push",
		"fetch",
		// "import",
		// "export",
		fmt.Sprintf("refspec %s", r.HeadRefs()),
		// fmt.Sprintf("refspec %s", r.BranchRefs()),
		fmt.Sprintf("refspec %s", r.TagRefs()),
		// fmt.Sprintf("*import-marks %s", gitMarks),
		// fmt.Sprintf("*export-marks %s", gitMarks),
	}
}

func (r *Repo) List() (map[string]string, error) {
	refs := map[string]string{}

	var recursiveFetch func(pathSlice []string) error

	recursiveFetch = func(pathSlice []string) error {
		valUncast, _, err := r.chaintree.ChainTree.Dag.Resolve(r.ctx, pathSlice)

		if err != nil {
			return err
		}

		switch val := valUncast.(type) {
		case map[string]interface{}:
			for key := range val {
				recursiveFetch(append(pathSlice, key))
			}
		case string:
			keyWithoutDataPrefix := strings.Join(pathSlice[2:], "/")
			refs[keyWithoutDataPrefix] = val
		}
		return nil
	}

	err := recursiveFetch([]string{"tree", "data", "refs"})

	return refs, err
}

func (r *Repo) Push(src string, dst string) error {
	if strings.HasPrefix(src, "+") {
		src = src[1:]
	}

	revParseOut, err := exec.Command("git", "rev-parse", src).Output()
	if err != nil {
		return err
	}

	srcRef := strings.TrimSpace(string(revParseOut))

	currentDestRef, err := r.resolveString(dst)
	if err != nil {
		return err
	}

	if srcRef == currentDestRef {
		return nil
	}

	// TODO: check existing dst hash
	revListOut, err := exec.Command("git", "rev-list", "--objects", src).Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSuffix(string(revListOut), "\n"), "\n")

	transactions := make([]*transactions.Transaction, len(lines)+1)

	for i, line := range lines {
		objectHash := strings.Split(line, " ")[0]

		objectType, err := exec.Command("git", "cat-file", "-t", objectHash).Output()
		if err != nil {
			return err
		}

		object, err := exec.Command("git", "cat-file", strings.TrimSpace(string(objectType)), objectHash).Output()
		if err != nil {
			return err
		}

		objectPath := []string{
			"objects",
			objectHash[0:2],
			objectHash[2:],
		}

		objectTxn, err := chaintree.NewSetDataTransaction(strings.Join(objectPath, "/"), object)
		if err != nil {
			return err
		}

		transactions[i] = objectTxn
	}

	refTxn, err := chaintree.NewSetDataTransaction(dst, srcRef)
	if err != nil {
		return err
	}

	transactions[len(transactions)-1] = refTxn

	_, err = r.tupelo.PlayTransactions(r.ctx, r.chaintree, r.privateKey, transactions)
	if err != nil {
		return err
	}

	err = r.setTip(r.chaintree.Tip())
	if err != nil {
		return err
	}

	return nil
}

func (r *Repo) Initialize() error {
	var err error

	if err = os.MkdirAll(r.Dir(), 0755); err != nil {
		return err
	}

	if err = r.touch("git.marks"); err != nil {
		return err
	}

	if err = r.touch(fmt.Sprintf("%s.marks", r.protocol)); err != nil {
		return err
	}

	r.tupelo, r.bitswap, err = NewTupeloClient(r.ctx, r.Dir())
	if err != nil {
		return err
	}

	repoKey, err := consensus.PassPhraseKey([]byte(r.Url()), []byte{11})
	if err != nil {
		return err
	}
	r.chaintree, err = consensus.NewSignedChainTree(r.ctx, repoKey.PublicKey, r.bitswap)

	currentTip, err := r.getTip()
	if err != nil {
		return err
	}

	if currentTip.Equals(blankCid) {
		// TOOD: User should register name or something probably vs making it magic on first fetch / push
		setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction([]string{crypto.PubkeyToAddress(r.privateKey.PublicKey).String()})
		if err != nil {
			return err
		}

		repoTxn, err := chaintree.NewSetDataTransaction("repo", r.Url())
		if err != nil {
			return err
		}

		transactions := []*transactions.Transaction{setOwnershipTxn, repoTxn}

		_, err = r.tupelo.PlayTransactions(r.ctx, r.chaintree, repoKey, transactions)
		if err != nil {
			return err
		}

		err = r.setTip(r.chaintree.Tip())
		if err != nil {
			return err
		}
	} else {
		r.chaintree.ChainTree.Dag = r.chaintree.ChainTree.Dag.WithNewTip(currentTip)
	}

	// fmt.Fprintf(os.Stderr, "chaintree %v\n", r.chaintree.ChainTree.Dag.Dump(r.ctx))

	return err
}

func (r *Repo) resolveString(path string) (string, error) {
	valUncast, _, err := r.chaintree.ChainTree.Dag.Resolve(r.ctx, append([]string{"tree", "data"}, strings.Split(path, "/")...))
	if valUncast == nil || err != nil {
		return "", err
	}
	return valUncast.(string), nil
}

func (r *Repo) setTip(tip cid.Cid) error {
	return ioutil.WriteFile(path.Join(r.Dir(), "tip"), []byte(tip.String()), 0755)
}

var blankCid = cid.Cid{}

func (r *Repo) getTip() (cid.Cid, error) {
	tip, err := ioutil.ReadFile(path.Join(r.Dir(), "tip"))
	if os.IsNotExist(err) {
		return blankCid, nil
	}

	if err != nil {
		return blankCid, err
	}

	tipCid, err := cid.Decode(string(tip))
	if err != nil {
		return blankCid, err
	}

	return tipCid, nil
}

func (r *Repo) touch(filename string) error {
	file, err := os.Create(path.Join(r.Dir(), filename))

	if os.IsExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return file.Close()
}
