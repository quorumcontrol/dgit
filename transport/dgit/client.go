package dgit

import (
	"context"
	"os"
	"path"

	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/nodestore"
	"github.com/quorumcontrol/decentragit-remote/tupelo/clientbuilder"
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

func NewClient(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	dir := path.Join(os.Getenv("GIT_DIR"), protocol)
	c.tupelo, c.nodestore, err = clientbuilder.Build(ctx, dir)
	return c, err
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

// FIXME: this probably shouldn't be here
func (c *Client) Tupelo() *tupelo.Client {
	return c.tupelo
}

// FIXME: this probably shouldn't be here
func (c *Client) Nodestore() nodestore.DagStore {
	return c.nodestore
}
