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
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"

	"github.com/quorumcontrol/dgit/constants"
	"github.com/quorumcontrol/dgit/tupelo/clientbuilder"
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

func Protocol() string {
	return constants.Protocol
}

func Default() (*Client, error) {
	client, ok := gitclient.Protocols[constants.Protocol]
	if !ok {
		return nil, fmt.Errorf("no client registered for '%s'", constants.Protocol)
	}

	asClient, ok := client.(*Client)
	if !ok {
		return nil, fmt.Errorf("%s registered %T, but is not a dgit.Tupelo", constants.Protocol, client)
	}

	return asClient, nil
}

func NewClient(ctx context.Context, basePath string) (*Client, error) {
	var err error
	c := &Client{ctx: ctx}
	dir := path.Join(basePath, constants.Protocol)
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
func (c *Client) CreateRepoTree(ctx context.Context, endpoint *transport.Endpoint, auth transport.AuthMethod) (*repotree.RepoTree, error) {
	var (
		pkAuth *PrivateKeyAuth
		ok     bool
	)
	if pkAuth, ok = auth.(*PrivateKeyAuth); !ok {
		return nil, fmt.Errorf("unable to cast %T to PrivateKeyAuth", auth)
	}
	return repotree.Create(ctx, &repotree.Options{
		Name:      endpoint.Host + endpoint.Path,
		Tupelo:    c.Tupelo,
		NodeStore: c.Nodestore,
		Owners:    []string{auth.String()},
	}, pkAuth.Key())
}

func (c *Client) FindRepoTree(ctx context.Context, repo string) (*repotree.RepoTree, error) {
	return repotree.Find(ctx, repo, c.Tupelo)
}

func (c *Client) RegisterAsDefault() {
	gitclient.InstallProtocol(constants.Protocol, c)
}

func (c *Client) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.Tupelo, c.Nodestore, auth)
	return server.NewServer(loader).NewUploadPackSession(ep, auth)
}

func (c *Client) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	loader := NewChainTreeLoader(c.ctx, c.Tupelo, c.Nodestore, auth)
	return server.NewServer(loader).NewReceivePackSession(ep, auth)
}

func (c *Client) AddRepoCollaborator(ctx context.Context, repo *Repo, collaborators []string) error {
	repoName, err := repo.Name()
	if err != nil {
		return err
	}

	repoTree, err := c.FindRepoTree(ctx, repoName)
	if err != nil {
		return err
	}

	auth, err := repo.Auth()
	if err != nil {
		return err
	}

	var (
		pkAuth *PrivateKeyAuth
		ok     bool
	)
	if pkAuth, ok = auth.(*PrivateKeyAuth); !ok {
		return fmt.Errorf("auth is not castable to PrivateKeyAuth; was a %T", auth)
	}

	return repoTree.AddCollaborators(ctx, collaborators, pkAuth.Key())
}

func (c *Client) ListRepoCollaborators(ctx context.Context, repo *Repo) ([]string, error) {
	repoName, err := repo.Name()
	if err != nil {
		return []string{}, err
	}

	repoTree, err := c.FindRepoTree(ctx, repoName)
	if err != nil {
		return []string{}, err
	}

	auth, err := repo.Auth()
	if err != nil {
		return []string{}, err
	}

	var (
		pkAuth *PrivateKeyAuth
		ok     bool
	)
	if pkAuth, ok = auth.(*PrivateKeyAuth); !ok {
		return []string{}, fmt.Errorf("could not cast auth to PrivateKeyAuth; was a %T", auth)
	}

	return repoTree.ListCollaborators(ctx, pkAuth.Key())
}

func (c *Client) RemoveRepoCollaborator(ctx context.Context, repo *Repo, collaborators []string) error {
	repoName, err := repo.Name()
	if err != nil {
		return err
	}

	repoTree, err := c.FindRepoTree(ctx, repoName)
	if err != nil {
		return err
	}

	auth, err := repo.Auth()
	if err != nil {
		return err
	}

	var (
		pkAuth *PrivateKeyAuth
		ok     bool
	)
	if pkAuth, ok = auth.(*PrivateKeyAuth); !ok {
		return fmt.Errorf("auth is not castable to PrivateKeyAuth; was a %T", auth)
	}

	return repoTree.RemoveCollaborators(ctx, collaborators, pkAuth.Key())
}
