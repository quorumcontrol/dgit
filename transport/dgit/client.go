package dgit

import (
	"context"
	"fmt"
	"path"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/chaintree/nodestore"
	tupelo "github.com/quorumcontrol/tupelo-go-sdk/gossip/client"

	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
	"github.com/quorumcontrol/dgit/tupelo/namedtree"
	"github.com/quorumcontrol/dgit/tupelo/repotree"
)

var log = logging.Logger("dgit.client")

type Client struct {
	transport.Transport

	ctx       context.Context
	Tupelo    *tupelo.Client
	Nodestore nodestore.DagStore
	server    transport.Transport
}

const protocol = "dgit"

func Protocol() string {
	return protocol
}

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

func NewClient(ctx context.Context, basePath string) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	dir := path.Join(basePath, protocol)
	c.Tupelo, c.Nodestore, err = clientbuilder.Build(ctx, dir)
	return c, err
}

// FIXME: this probably shouldn't be here
func NewLocalClient(ctx context.Context) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	c.Tupelo, c.Nodestore, err = clientbuilder.BuildLocal(ctx)
	return c, err
}

// FIXME: this probably shouldn't be here
func (c *Client) CreateRepoTree(ctx context.Context, endpoint *transport.Endpoint, auth transport.AuthMethod) (*namedtree.NamedTree, error) {
	return repotree.Create(ctx, &repotree.Options{
		Options: &namedtree.Options{
			Name:      endpoint.Host + endpoint.Path,
			Client:    c.Tupelo,
			NodeStore: c.Nodestore,
			Owners:    []string{auth.String()},
		},
	})
}

func (c *Client) FindRepoTree(ctx context.Context, repo string) (*namedtree.NamedTree, error) {
	return repotree.Find(ctx, repo, c.Tupelo)
}

func (c *Client) RegisterAsDefault() {
	gitclient.InstallProtocol(protocol, c)
}

func (c *Client) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.Tupelo, c.Nodestore, auth)
	return server.NewServer(loader).NewUploadPackSession(ep, auth)
}

func (c *Client) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.Tupelo, c.Nodestore, auth)
	return server.NewServer(loader).NewReceivePackSession(ep, auth)
}
