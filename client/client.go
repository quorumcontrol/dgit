package client

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"github.com/quorumcontrol/tupelo-go-sdk/p2p"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitclient "gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

const userPrivateKey = "0x5643765d2b05b1b8fc6d5419c88aa0a5be8ef410e8ebc9f4e0ce752334ff2f33"

var log = logging.Logger("dgit.client")

type Client struct {
	transport.Transport

	tupelo  *tupelo.Client
	bitswap *p2p.BitswapPeer
}

const protocol = "dgit"

func New(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{}
	dir := path.Join(os.Getenv("GIT_DIR"), protocol)
	c.tupelo, c.bitswap, err = NewTupeloClient(ctx, dir)
	return c, err
}

func (c *Client) RegisterAsDefault() {
	gitclient.InstallProtocol(protocol, c)
}

type Session struct {
	transport.UploadPackSession
	transport.ReceivePackSession

	ep     *transport.Endpoint
	auth   transport.AuthMethod
	client *Client
}

func (s *Session) privateKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	// TODO: move to secure storage, using transport.AuthMethod
	privateKeyBytes, err := hexutil.Decode(userPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding user private key: %v", err)
	}

	ecdsaPrivate, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal ECDSA private key: %v", err)
	}
	return ecdsaPrivate, nil
}

func (s *Session) ChainTree(ctx context.Context) (*consensus.SignedChainTree, error) {
	chainTreeKey, err := consensus.PassPhraseKey([]byte(s.ep.Host), []byte(s.ep.Path))
	if err != nil {
		return nil, err
	}

	chainTree, err := consensus.NewSignedChainTree(ctx, chainTreeKey.PublicKey, s.client.bitswap)
	if err != nil {
		return nil, err
	}

	log.Infof("repo chaintree: %s", chainTree.MustId())

	err = s.client.tupelo.WaitForFirstRound(ctx, 10*time.Second)
	if err != nil {
		return nil, err
	}

	proof, err := s.client.tupelo.GetTip(ctx, chainTree.MustId())
	if err == tupelo.ErrNotFound || proof == nil {
		privateKey, err := s.privateKey(ctx)
		if err != nil {
			return nil, err
		}

		// TOOD: User should register name or something probably vs making it magic on first fetch / push
		setOwnershipTxn, err := chaintree.NewSetOwnershipTransaction([]string{crypto.PubkeyToAddress(privateKey.PublicKey).String()})
		if err != nil {
			return nil, err
		}

		repoTxn, err := chaintree.NewSetDataTransaction("repo", strings.Join([]string{s.ep.Host, s.ep.Path}, "/"))
		if err != nil {
			return nil, err
		}

		transactions := []*transactions.Transaction{setOwnershipTxn, repoTxn}

		_, err = s.client.tupelo.PlayTransactions(ctx, chainTree, chainTreeKey, transactions)
		if err != nil {
			return nil, err
		}

		return chainTree, nil
	}

	if err != nil {
		return nil, err
	}

	tipCid, err := cid.Parse(proof.Tip)
	if err != nil {
		return nil, err
	}

	chainTree.ChainTree.Dag = chainTree.ChainTree.Dag.WithNewTip(tipCid)

	return chainTree, nil
}

func (s *Session) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	log.Debugf("received UploadPack")
	fmt.Fprintln(os.Stderr)
	spew.Fdump(os.Stderr, "UploadPack")
	spew.Fdump(os.Stderr, "packp.UploadPackRequest")
	spew.Fdump(os.Stderr, req)

	return nil, nil
}

func (s *Session) ReceivePack(ctx context.Context, req *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error) {
	var err error

	log.Debugf("ReceivePack for %s", s.ep.String())

	chainTree, err := s.ChainTree(ctx)
	if err != nil {
		return nil, err
	}

	log.Debugf("reading packfile")
	r := ioutil.NewContextReadCloser(ctx, req.Packfile)

	if r == nil {
		return nil, nil
	}

	path := "/Users/bwestcott/working/quorumcontrol/decentragit-remote/test-git"

	if err = os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	localStorer := filesystem.NewStorage(osfs.New(path), cache.NewObjectLRUDefault())

	log.Debugf("loading packfile into storage")
	if err := packfile.UpdateObjectStorage(localStorer, r); err != nil {
		_ = r.Close()
		return nil, err
	}

	transactions := []*transactions.Transaction{}
	cmdStatuses := []*packp.CommandStatus{}

	for _, cmd := range req.Commands {
		var val interface{}

		ref := plumbing.NewHashReference(cmd.Name, cmd.New)

		log.Debugf("processing command %s for %s", cmd.Action(), ref.String())

		if cmd.Action() == packp.Delete {
			val = nil
		} else {
			val = ref.Hash()
		}

		txn, err := chaintree.NewSetDataTransaction(string(ref.Name()), val)
		if err != nil {
			return nil, err
		}

		log.Debugf("chaintree transaction: %v", txn)

		transactions = append(transactions, txn)
		cmdStatuses = append(cmdStatuses, &packp.CommandStatus{
			ReferenceName: ref.Name(),
			Status:        "ok",
		})
	}

	privateKey, err := s.privateKey(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.client.tupelo.PlayTransactions(ctx, chainTree, privateKey, transactions)
	if err != nil {
		return nil, err
	}

	return &packp.ReportStatus{
		CommandStatuses: cmdStatuses,
	}, nil

	// iterator, err := storer.IterEncodedObjects(plumbing.AnyObject)
	// if err != nil {
	// 	panic(err)
	// }

	// iterator.ForEach(func(obj plumbing.EncodedObject) error {
	// 	fmt.Fprintln(os.Stderr)
	// 	fmt.Fprintf(os.Stderr, obj.Hash().String())
	// 	fmt.Fprintln(os.Stderr)
	// 	all, _ := obj.Reader()
	// 	allStr, _ := goioutil.ReadAll(all)
	// 	fmt.Fprintf(os.Stderr, string(allStr))
	// 	fmt.Fprintln(os.Stderr)
	// 	return nil
	// })

	// err := storer.ForEachObjectHash(func(h plumbing.Hash) error {
	// 	spew.Fdump(os.Stderr, h)
	// 	// obj, err := storer.EncodedObject(plumbing.AnyObject, h)
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// 	// // spew.Fdump(os.Stderr, obj)
	// 	return nil
	// })
	// if err != nil {
	// 	panic(err)
	// }

	// readAll, err := ioutil.ReadAll(req.Packfile)
	// if err != nil {
	// 	panic(err)
	// }
	// spew.Fdump(os.Stderr, readAll)

	return nil, nil
}

func (s *Session) AdvertisedReferences() (*packp.AdvRefs, error) {
	log.Debugf("AdvertisedReferences for %s", s.ep.String())

	chainTree, err := s.ChainTree(context.Background())
	if err != nil {
		return nil, err
	}

	ar := packp.NewAdvRefs()

	var recursiveFetch func(pathSlice []string) error

	recursiveFetch = func(pathSlice []string) error {
		valUncast, _, err := chainTree.ChainTree.Dag.Resolve(context.Background(), pathSlice)

		if err != nil {
			return err
		}

		switch val := valUncast.(type) {
		case map[string]interface{}:
			for key := range val {
				recursiveFetch(append(pathSlice, key))
			}
		case string:
			refName := plumbing.ReferenceName(strings.Join(pathSlice[2:], "/"))
			ref := plumbing.NewHashReference(refName, plumbing.NewHash(val))

			err = ar.AddReference(ref)
			if err != nil {
				return err
			}
			log.Debugf("adding reference: %s, hash: %s", ref.Name(), ref.Hash())
		}
		return nil
	}

	err = recursiveFetch([]string{"tree", "data", "refs"})

	log.Debugf("references: %v", ar.References)
	log.Debugf("capabilities: %v", ar.Capabilities.String())

	return ar, err
}

func (s *Session) Close() error {
	log.Debugf("session for %s closed", s.ep.String())
	return nil
}

func (c *Client) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	log.Debugf("NewUploadPackSession started for %s with auth %T", ep.String(), auth)
	return &Session{
		ep:     ep,
		auth:   auth,
		client: c,
	}, nil
}

func (c *Client) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	log.Debugf("NewReceivePackSession started for %s with auth %T", ep.String(), auth)
	return &Session{
		ep:     ep,
		auth:   auth,
		client: c,
	}, nil
}
