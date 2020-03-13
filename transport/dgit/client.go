package dgit

import (
	"context"
	"fmt"
	"os"
	"path"

	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
	"github.com/quorumcontrol/dgit/tupelo/repotree"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitclient "gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/server"
)

var log = logging.Logger("dgit.client")

type Client struct {
	transport.Transport

	ctx       context.Context
	tupelo    *tupelo.Client
	nodestore nodestore.DagStore
	server    transport.Transport
}

const protocol = "dgit"

func Default() (*Client, error) {
	client, ok := gitclient.Protocols[protocol]
	if !ok {
		return nil, fmt.Errorf("no client registered for '%s'", protocol)
	}

	asClient, ok := client.(*Client)
	if !ok {
		return nil, fmt.Errorf("%s registered %T, but is not a dgit.Client", protocol, client)
	}

	return asClient, nil
}

func NewClient(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	dir := path.Join(os.Getenv("GIT_DIR"), protocol)
	c.tupelo, c.nodestore, err = clientbuilder.Build(ctx, dir)
	return c, err
}

// FIXME: this probably shouldn't be here
func NewLocalClient(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	c.tupelo, c.nodestore, err = clientbuilder.BuildLocal(ctx)
	return c, err
}

// FIXME: this probably shouldn't be here
func (c *Client) CreateRepoTree(ctx context.Context, endpoint *transport.Endpoint, auth transport.AuthMethod) (*consensus.SignedChainTree, error) {
	// TODO: When `dgit` or prompt  exists; allow configuring this w/ CLI args, API options, etc.
	objStorageType := os.Getenv("DGIT_OBJ_STORAGE")
	return repotree.Create(ctx, &repotree.RepoTreeOptions{
		Name:              endpoint.Host + "/" + endpoint.Path,
		ObjectStorageType: objStorageType,
		Client:            c.tupelo,
		NodeStore:         c.nodestore,
		Ownership:         []string{auth.String()},
	})
}

func (c *Client) RegisterAsDefault() {
	gitclient.InstallProtocol(protocol, c)
}

func (c *Client) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.tupelo, c.nodestore, auth)
	return server.NewServer(loader).NewUploadPackSession(ep, auth)
}

func (c *Client) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.tupelo, c.nodestore, auth)
	return server.NewServer(loader).NewReceivePackSession(ep, auth)
}
