package initializer

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/manifoldco/promptui"
	"github.com/quorumcontrol/dgit/keyring"
	"github.com/quorumcontrol/dgit/msg"
	"github.com/quorumcontrol/dgit/transport/dgit"
	"github.com/quorumcontrol/dgit/tupelo/repotree"
	"github.com/quorumcontrol/tupelo-go-sdk/consensus"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const dgitRemote = "dgit"

var validRepoName = regexp.MustCompile(`^[a-zA-Z0-9_-]+\/[a-zA-Z0-9_-]+`)

var promptTemplates = &promptui.PromptTemplates{
	Prompt:  `{{ . }} `,
	Confirm: `{{ . }} {{ "Y/n" | bold }} `,
	Valid:   `{{ . }} `,
	Invalid: `{{ . }} `,
	Success: `{{ . }} `,
}

type Initializer struct {
	stdin    io.ReadCloser
	stdout   io.WriteCloser
	stderr   io.WriteCloser
	keyring  keyring.Keyring
	auth     transport.AuthMethod
	repo     *git.Repository
	endpoint *transport.Endpoint
}

func Init(ctx context.Context, repo *git.Repository, args []string) error {
	i := &Initializer{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		repo:   repo,
	}
	return i.Init(ctx, args)
}

func (i *Initializer) Init(ctx context.Context, args []string) error {
	var err error

	// load up auth and notify user if new
	_, err = i.getAuth()
	if err != nil {
		return err
	}

	// determine endpoint, prompt user as needed
	endpoint, err := i.getEndpoint()
	if err != nil {
		return err
	}

	_, err = i.findOrCreateRepoTree(ctx)
	if err != nil {
		return err
	}

	err = i.addDgitPushToRemote(ctx, "origin")
	if err != nil {
		return err
	}

	err = i.addDgitRemote(ctx)
	if err != nil {
		return err
	}

	msg.Fprint(i.stdout, msg.FinalInstructions, map[string]interface{}{
		"repo":    repoNameFor(endpoint),
		"repourl": endpoint.String(),
	})
	fmt.Fprintln(os.Stdout)

	return nil
}

func (i *Initializer) getAuth() (transport.AuthMethod, error) {
	if i.auth != nil {
		return i.auth, nil
	}

	kr, err := keyring.NewDefault()
	// TODO: if no keyring available, prompt user for dgit password
	if err != nil {
		return nil, fmt.Errorf("Error with keyring: %v", err)
	}

	privateKey, isNew, err := keyring.FindOrCreatePrivateKey(kr)
	if err != nil {
		return nil, fmt.Errorf("Error fetching key from keyring: %v", err)
	}

	if isNew {
		msg.Fprint(i.stdout, msg.Welcome, map[string]interface{}{
			"keyringProvider": keyring.Name(kr),
			"userAddress":     crypto.PubkeyToAddress(privateKey.PublicKey).String(),
		})
		fmt.Fprintln(i.stdout)
	}

	i.auth = dgit.NewPrivateKeyAuth(privateKey)
	return i.auth, nil
}

func (i *Initializer) findOrCreateRepoTree(ctx context.Context) (*consensus.SignedChainTree, error) {
	client, err := dgit.Default()
	if err != nil {
		return nil, err
	}

	endpoint, err := i.getEndpoint()
	if err != nil {
		return nil, err
	}

	tree, err := client.FindRepoTree(ctx, repoNameFor(endpoint))
	// repo already exists, return out
	if err == nil && tree != nil {
		return tree, nil
	}
	// a real error, return
	if err != repotree.ErrNotFound {
		return nil, err
	}

	auth, err := i.getAuth()
	if err != nil {
		return nil, err
	}

	// repo doesn't exist, create it
	newTree, err := client.CreateRepoTree(ctx, endpoint, auth, "siaskynet")
	if err != nil {
		return nil, err
	}

	msg.Fprint(i.stdout, msg.RepoCreated, map[string]interface{}{
		"did":  newTree.MustId(),
		"repo": repoNameFor(endpoint),
	})
	fmt.Fprintln(i.stdout)

	return newTree, nil
}

func (i *Initializer) getEndpoint() (*transport.Endpoint, error) {
	var err error
	if i.endpoint == nil {
		i.endpoint, err = i.determineDgitEndpint()
	}
	return i.endpoint, err
}

func (i *Initializer) determineDgitEndpint() (*transport.Endpoint, error) {
	var dgitEndpoint *transport.Endpoint

	remotes, err := i.repo.Remotes()
	if err != nil {
		return nil, err
	}

	// get remotes sorted by dgit, then origin, then rest
	sort.Slice(remotes, func(i, j int) bool {
		iName := remotes[i].Config().Name
		jName := remotes[j].Config().Name
		if iName == "origin" && jName == dgitRemote {
			return false
		}
		return iName == "origin" || iName == dgitRemote
	})

	dgitUrls := []string{}

	for _, remote := range remotes {
		for _, url := range remote.Config().URLs {
			if strings.HasPrefix(url, dgit.Protocol()) {
				dgitUrls = append(dgitUrls, url)
			}
		}
	}

	if len(dgitUrls) > 0 {
		return transport.NewEndpoint(dgitUrls[0])
	}

	if len(remotes) > 0 {
		otherEndpoint, err := transport.NewEndpoint(remotes[0].Config().URLs[0])
		if err != nil {
			return nil, err
		}

		repoFullPath := strings.ToLower(strings.TrimSuffix(otherEndpoint.Path, ".git"))
		repoUser := strings.Split(repoFullPath, "/")[0]
		repoName := strings.TrimPrefix(repoFullPath, repoUser+"/")
		dgitEndpoint, err = newDgitEndpoint(repoUser, repoName)
		if err != nil {
			return nil, err
		}

		prompt := promptui.Prompt{
			Label: stripNewLines(msg.Parse(msg.PromptRepoNameConfirm, map[string]interface{}{
				"remote": "origin",
				"repo":   repoNameFor(dgitEndpoint),
			})),
			Templates: promptTemplates,
			IsConfirm: true,
			Default:   "y",
			Stdin:     i.stdin,
			Stdout:    i.stdout,
		}
		_, err = prompt.Run()
		fmt.Fprintln(i.stdout)
		// if err is abort, continue on below
		if err != promptui.ErrAbort {
			return dgitEndpoint, err
		}
	}

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.PromptRepoName),
		Templates: promptTemplates,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
		Validate: func(input string) error {
			if !validRepoName.MatchString(input) {
				return fmt.Errorf(stripNewLines(msg.PromptRepoNameInvalid))
			}
			return nil
		},
	}

	result, err := prompt.Run()
	fmt.Fprintln(i.stdout)
	if err != nil {
		return nil, err
	}
	result = strings.ToLower(result)
	repoUser := strings.Split(result, "/")[0]
	repoName := strings.TrimPrefix(result, repoUser+"/")
	return newDgitEndpoint(repoUser, repoName)
}

func (i *Initializer) addDgitPushToRemote(ctx context.Context, remoteName string) error {
	remote, err := i.repo.Remote(remoteName)

	if err == git.ErrRemoteNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	remoteConfig := remote.Config()

	endpoint, err := i.getEndpoint()
	if err != nil {
		return err
	}

	for _, url := range remoteConfig.URLs {
		if url == endpoint.String() {
			// already has dgit url, no need to add another
			return nil
		}
	}

	msg.Fprint(i.stdout, msg.AddDgitToRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    repoNameFor(endpoint),
		"repourl": endpoint.String(),
	})
	fmt.Fprintln(i.stdout)

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.AddDgitToRemoteConfirm),
		Default:   "y",
		Templates: promptTemplates,
		IsConfirm: true,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
	}
	_, err = prompt.Run()
	fmt.Fprintln(i.stdout)
	if err != nil && err != promptui.ErrAbort {
		return err
	}
	// user doesn't want dgit in origin, return out :(
	if err == promptui.ErrAbort {
		return nil
	}

	remoteConfig.URLs = append(remoteConfig.URLs, endpoint.String())

	newConfig, err := i.repo.Config()
	if err != nil {
		return err
	}
	newConfig.Remotes[remoteConfig.Name] = remoteConfig
	err = newConfig.Validate()
	if err != nil {
		return err
	}

	err = i.repo.Storer.SetConfig(newConfig)
	if err != nil {
		return err
	}

	msg.Fprint(i.stdout, msg.AddedDgitToRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    repoNameFor(endpoint),
		"repourl": endpoint.String(),
	})
	fmt.Fprintln(i.stdout)
	return nil
}

func (i *Initializer) addDgitRemote(ctx context.Context) error {
	_, err := i.repo.Remote(dgitRemote)
	if err != git.ErrRemoteNotFound {
		return err
	}

	endpoint, err := i.getEndpoint()
	if err != nil {
		return err
	}

	remoteConfig := &config.RemoteConfig{
		Name: dgitRemote,
		URLs: []string{endpoint.String()},
	}
	err = remoteConfig.Validate()
	if err != nil {
		return err
	}

	msg.Fprint(i.stdout, msg.AddDgitRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    repoNameFor(endpoint),
		"repourl": endpoint.String(),
	})
	fmt.Fprintln(i.stdout)

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.AddDgitRemoteConfirm),
		Default:   "y",
		Templates: promptTemplates,
		IsConfirm: true,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
	}
	_, err = prompt.Run()
	fmt.Fprintln(i.stdout)
	// user doesn't want dgit remote, return out :(
	if err == promptui.ErrAbort {
		return nil
	}
	if err != nil {
		return err
	}

	newConfig, err := i.repo.Config()
	if err != nil {
		return err
	}
	newConfig.Remotes[remoteConfig.Name] = remoteConfig
	err = newConfig.Validate()
	if err != nil {
		return err
	}

	err = i.repo.Storer.SetConfig(newConfig)
	if err != nil {
		return err
	}

	msg.Fprint(i.stdout, msg.AddedDgitRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    repoNameFor(endpoint),
		"repourl": endpoint.String(),
	})
	fmt.Fprintln(i.stdout)
	return nil
}

func newDgitEndpoint(user string, repo string) (*transport.Endpoint, error) {
	// the New(String()) is for parsing validation
	return transport.NewEndpoint((&transport.Endpoint{
		Protocol: dgit.Protocol(),
		Host:     user,
		Path:     repo,
	}).String())
}

func repoNameFor(e *transport.Endpoint) string {
	return e.Host + e.Path
}

func stripNewLines(s string) string {
	replacement := " "
	return strings.TrimSpace(strings.NewReplacer(
		"\r\n", replacement,
		"\r", replacement,
		"\n", replacement,
		"\v", replacement,
		"\f", replacement,
		"\u0085", replacement,
		"\u2028", replacement,
		"\u2029", replacement,
	).Replace(s))
}
