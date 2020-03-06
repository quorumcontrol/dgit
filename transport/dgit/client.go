package dgit

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/chaintree"
	"github.com/quorumcontrol/chaintree/nodestore"
	chaintreestore "github.com/quorumcontrol/decentragit-remote/storage/chaintree"
	"github.com/quorumcontrol/decentragit-remote/tupelo/clientbuilder"
	"github.com/quorumcontrol/decentragit-remote/tupelo/repotree"
	"github.com/quorumcontrol/messages/v2/build/go/transactions"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
	"gopkg.in/src-d/go-git.v4/plumbing/revlist"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitclient "gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/server"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

const userPrivateKey = "0x5643765d2b05b1b8fc6d5419c88aa0a5be8ef410e8ebc9f4e0ce752334ff2f33"

var log = logging.Logger("dgit.client")

type Client struct {
	transport.Transport
	tupelo    *tupelo.Client
	nodestore nodestore.DagStore
}

const protocol = "dgit"

func NewClient(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{}
	dir := path.Join(os.Getenv("GIT_DIR"), protocol)
	c.tupelo, c.nodestore, err = clientbuilder.Build(ctx, dir)
	return c, err
}

func (c *Client) RegisterAsDefault() {
	gitclient.InstallProtocol(protocol, c)
}

// FIXME: this probably shouldn't be here
func (c *Client) Tupelo() *tupelo.Client {
	return c.tupelo
}

// FIXME: this probably shouldn't be here
func (c *Client) Nodestore() nodestore.DagStore {
	return c.nodestore
}

type Session struct {
	transport.UploadPackSession
	transport.ReceivePackSession

	caps   *capability.List
	ep     *transport.Endpoint
	auth   transport.AuthMethod
	client *Client
	// true for fetch ops, false for push ops
	asClient bool
}

func FIXMETemporaryPrivateKey() (*ecdsa.PrivateKey, error) {
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

func (s *Session) objectStorage(ctx context.Context, chainTree *consensus.SignedChainTree) (storer.EncodedObjectStorer, error) {
	treeKey, err := FIXMETemporaryPrivateKey()
	if err != nil {
		return nil, err
	}
	return chaintreestore.NewObjectStorage(ctx, s.client.tupelo, chainTree, treeKey), nil
}

// UploadPack is sending FROM the ChainTree TO the local repo, aka `git fetch / git clone`
func (s *Session) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	log.Debugf("UploadPack for %s", s.ep.String())

	if req.IsEmpty() {
		return nil, transport.ErrEmptyUploadPackRequest
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	if s.caps == nil {
		s.caps = capability.NewList()
		if err := s.setSupportedCapabilities(s.caps); err != nil {
			return nil, err
		}
	}

	if err := s.checkSupportedCapabilities(req.Capabilities); err != nil {
		return nil, err
	}

	s.caps = req.Capabilities

	if len(req.Shallows) > 0 {
		return nil, fmt.Errorf("shallow not supported")
	}

	chainTree, err := repotree.Find(ctx, s.ep.Host+"/"+s.ep.Path, s.client.tupelo)
	if err == repotree.ErrNotFound {
		return nil, transport.ErrRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}

	localStorer, err := s.objectStorage(ctx, chainTree)
	if err != nil {
		return nil, err
	}

	objs, err := s.objectsToUpload(req, localStorer)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	e := packfile.NewEncoder(pw, localStorer, false)
	go func() {
		// TODO: plumb through a pack window.
		_, err := e.Encode(objs, 10)
		pw.CloseWithError(err)
	}()

	return packp.NewUploadPackResponseWithPackfile(req,
		ioutil.NewContextReadCloser(ctx, pr),
	), nil
}

func (s *Session) objectsToUpload(req *packp.UploadPackRequest, storer storer.EncodedObjectStorer) ([]plumbing.Hash, error) {
	haves, err := revlist.Objects(storer, req.Haves, nil)
	if err != nil {
		return nil, err
	}

	return revlist.Objects(storer, req.Wants, haves)
}

// ReceivePack is sending FROM the local repo TO the ChainTree, aka `git push`
func (s *Session) ReceivePack(ctx context.Context, req *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error) {
	var err error

	log.Debugf("ReceivePack for %s", s.ep.String())

	if s.caps == nil {
		s.caps = capability.NewList()
		if err := s.setSupportedCapabilities(s.caps); err != nil {
			return nil, err
		}
	}

	if err := s.checkSupportedCapabilities(req.Capabilities); err != nil {
		return nil, err
	}

	chainTree, err := repotree.Find(ctx, s.ep.Host+"/"+s.ep.Path, s.client.tupelo)
	if err == repotree.ErrNotFound {
		return nil, transport.ErrRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}

	log.Debugf("reading packfile")
	r := ioutil.NewContextReadCloser(ctx, req.Packfile)

	if r == nil {
		return nil, nil
	}

	localStorer, err := s.objectStorage(ctx, chainTree)
	if err != nil {
		return nil, err
	}

	rs := packp.NewReportStatus()
	rs.UnpackStatus = "ok"

	log.Debugf("loading packfile into storage")
	p, err := packfile.NewParserWithStorage(packfile.NewScanner(r), localStorer)
	if err != nil {
		_ = r.Close()
		rs.UnpackStatus = err.Error()
		return rs, err
	}

	_, err = p.Parse()
	if err != nil {
		_ = r.Close()
		rs.UnpackStatus = err.Error()
		return rs, err
	}

	transactions := []*transactions.Transaction{}
	cmdStatuses := []*packp.CommandStatus{}

	for _, cmd := range req.Commands {
		var val interface{}

		ref := plumbing.NewHashReference(cmd.Name, cmd.New)

		log.Debugf("processing command %s for %s", cmd.Action(), ref.String())

		refChainTreePath := append([]string{"tree", "data"}, strings.Split(ref.Name().String(), "/")...)

		currentRef, _, err := chainTree.ChainTree.Dag.Resolve(ctx, refChainTreePath)

		if err != nil && err != format.ErrNotFound {
			log.Errorf("could not resolve current ref for %s: %v", ref.Name().String(), err)
			continue
		}

		log.Debugf("current ref for %s is %v", ref.Name().String(), currentRef)

		switch cmd.Action() {
		case packp.Delete:
			if currentRef == nil {
				cmdStatuses = append(cmdStatuses, &packp.CommandStatus{
					ReferenceName: ref.Name(),
					Status:        server.ErrUpdateReference.Error(),
				})
				continue
			}
			val = nil
		case packp.Create:
			if currentRef != nil {
				cmdStatuses = append(cmdStatuses, &packp.CommandStatus{
					ReferenceName: ref.Name(),
					Status:        server.ErrUpdateReference.Error(),
				})
				continue
			}

			val = ref.Hash().String()
		case packp.Update:
			if currentRef == nil {
				cmdStatuses = append(cmdStatuses, &packp.CommandStatus{
					ReferenceName: ref.Name(),
					Status:        server.ErrUpdateReference.Error(),
				})
				continue
			}

			val = ref.Hash().String()
		}

		txn, err := chaintree.NewSetDataTransaction(ref.Name().String(), val)
		if err != nil {
			return rs, err
		}

		log.Debugf("chaintree transaction: %v", txn)

		transactions = append(transactions, txn)
		cmdStatuses = append(cmdStatuses, &packp.CommandStatus{
			ReferenceName: ref.Name(),
			Status:        "ok",
		})
	}

	privateKey, err := FIXMETemporaryPrivateKey()
	if err != nil {
		return rs, err
	}

	log.Debugf("playing transactions %v", transactions)
	proof, err := s.client.tupelo.PlayTransactions(ctx, chainTree, privateKey, transactions)
	if err != nil {
		return rs, err
	}

	tipCid, err := cid.Parse(proof.Tip)
	if err != nil {
		return rs, err
	}

	log.Debugf("new chaintree tip %s", tipCid.String())

	rs.CommandStatuses = cmdStatuses

	if req.Capabilities.Supports(capability.ReportStatus) {
		return rs, nil
	} else {
		return nil, rs.Error()
	}
}

func (s *Session) setSupportedCapabilities(c *capability.List) error {
	if err := c.Set(capability.Agent, capability.DefaultAgent); err != nil {
		return err
	}

	if err := c.Set(capability.OFSDelta); err != nil {
		return err
	}

	if !s.asClient {
		if err := c.Set(capability.ReportStatus); err != nil {
			return err
		}

		if err := c.Set(capability.DeleteRefs); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) checkSupportedCapabilities(cl *capability.List) error {
	for _, c := range cl.All() {
		if !s.caps.Supports(c) {
			return fmt.Errorf("unsupported capability: %s", c)
		}
	}

	return nil
}

func (s *Session) AdvertisedReferences() (*packp.AdvRefs, error) {
	log.Debugf("AdvertisedReferences for %s", s.ep.String())

	ar := packp.NewAdvRefs()

	if err := s.setSupportedCapabilities(ar.Capabilities); err != nil {
		return nil, err
	}

	s.caps = ar.Capabilities

	chainTree, err := repotree.Find(context.Background(), s.ep.Host+"/"+s.ep.Path, s.client.tupelo)
	if err == repotree.ErrNotFound {
		return nil, transport.ErrRepositoryNotFound
	}
	if err != nil {
		return nil, err
	}

	var recursiveFetch func(pathSlice []string) error

	recursiveFetch = func(pathSlice []string) error {
		log.Debugf("fetching references under: %s", pathSlice)

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
			log.Debugf("ref name is: %s", refName)
			log.Debugf("val is: %s", val)
			ref := plumbing.NewHashReference(refName, plumbing.NewHash(val))

			err = ar.AddReference(ref)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err = recursiveFetch([]string{"tree", "data", "refs"})

	log.Debugf("references: %v", ar.References)
	log.Debugf("capabilities: %v", ar.Capabilities.String())

	if s.asClient && len(ar.References) == 0 {
		return ar, transport.ErrEmptyRemoteRepository
	}

	return ar, err
}

func (s *Session) Close() error {
	log.Debugf("session for %s closed", s.ep.String())
	return nil
}

func (c *Client) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	log.Debugf("NewUploadPackSession started for %s with auth %T", ep.String(), auth)
	return &Session{
		ep:       ep,
		auth:     auth,
		client:   c,
		asClient: true,
	}, nil
}

func (c *Client) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	log.Debugf("NewReceivePackSession started for %s with auth %T", ep.String(), auth)
	return &Session{
		ep:       ep,
		auth:     auth,
		client:   c,
		asClient: false,
	}, nil
}
