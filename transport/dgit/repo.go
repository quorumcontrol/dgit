package dgit

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/quorumcontrol/dgit/constants"
	"github.com/quorumcontrol/dgit/keyring"
)

var ErrEndpointNotFound = errors.New("endpoint not found")

type Repo struct {
	*git.Repository

	endpoint *transport.Endpoint
	auth     transport.AuthMethod
}

func NewRepo(gitRepo *git.Repository) *Repo {
	return &Repo{Repository: gitRepo}
}

func (r *Repo) Endpoint() (*transport.Endpoint, error) {
	if r.endpoint != nil {
		return r.endpoint, nil
	}

	remotes, err := r.Repository.Remotes()
	if err != nil {
		return nil, err
	}

	// get remotes sorted by dgit, then origin, then rest
	sort.Slice(remotes, func(i, j int) bool {
		iName := remotes[i].Config().Name
		jName := remotes[j].Config().Name
		if iName == "origin" && jName == constants.DgitRemote {
			return false
		}
		return iName == "origin" || iName == constants.DgitRemote
	})

	dgitUrls := []string{}

	for _, remote := range remotes {
		for _, url := range remote.Config().URLs {
			if strings.HasPrefix(url, constants.Protocol) {
				dgitUrls = append(dgitUrls, url)
			}
		}
	}

	if len(dgitUrls) > 0 {
		ep, err := transport.NewEndpoint(dgitUrls[0])
		if err != nil {
			return nil, err
		}

		r.endpoint = ep

		return ep, nil
	}

	return nil, ErrEndpointNotFound
}

func (r *Repo) MustEndpoint() *transport.Endpoint {
	ep, err := r.Endpoint()
	if err != nil {
		panic(err)
	}

	return ep
}

func (r *Repo) SetEndpoint(endpoint *transport.Endpoint) {
	r.endpoint = endpoint
}

func (r *Repo) Name() (string, error) {
	ep, err := r.Endpoint()
	if err != nil {
		return "", err
	}

	return ep.Host + ep.Path, nil
}

func (r *Repo) MustName() string {
	name, err := r.Name()
	if err != nil {
		panic(err)
	}

	return name
}

func (r *Repo) URL() (string, error) {
	ep, err := r.Endpoint()
	if err != nil {
		return "", err
	}

	return ep.String(), nil
}

func (r *Repo) MustURL() string {
	url, err := r.URL()
	if err != nil {
		panic(err)
	}

	return url
}

func (r *Repo) Username() (string, error) {
	repoConfig, err := r.Config()
	if err != nil {
		return "", err
	}

	dgitConfig := repoConfig.Merged.Section(constants.DgitConfigSection)

	if dgitConfig == nil {
		return "", fmt.Errorf("no decentragit configuration found; run `git config --global %s.username your-username`", constants.DgitConfigSection)
	}

	username := dgitConfig.Option("username")

	envUsername := os.Getenv("DGIT_USERNAME")
	if envUsername != "" {
		log.Warningf("[DEPRECATION] - DGIT_USERNAME is deprecated, please use DG_USERNAME")
		username = envUsername
	}
	envUsername = os.Getenv("DG_USERNAME")
	if envUsername != "" {
		username = envUsername
	}

	if username == "" {
		return "", fmt.Errorf("no decentragit username found; run `git config --global %s.username your-username`", constants.DgitConfigSection)
	}

	return username, nil
}

func (r *Repo) Auth() (transport.AuthMethod, error) {
	if r.auth != nil {
		return r.auth, nil
	}

	kr, err := keyring.NewDefault()
	if err != nil {
		return nil, err
	}

	username, err := r.Username()
	if err != nil {
		return nil, err
	}

	privateKey, err := kr.FindPrivateKey(username)
	if err != nil {
		return nil, err
	}

	r.auth = NewPrivateKeyAuth(privateKey)

	return r.auth, nil
}
