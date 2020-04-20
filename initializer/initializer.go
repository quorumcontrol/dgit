package initializer

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	configformat "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	logging "github.com/ipfs/go-log"
	"github.com/manifoldco/promptui"
	"github.com/quorumcontrol/chaintree/nodestore"
	tupelo "github.com/quorumcontrol/tupelo/sdk/gossip/client"
	"github.com/tyler-smith/go-bip39"

	"github.com/quorumcontrol/dgit/constants"
	"github.com/quorumcontrol/dgit/keyring"
	"github.com/quorumcontrol/dgit/msg"
	"github.com/quorumcontrol/dgit/transport/dgit"
	"github.com/quorumcontrol/dgit/tupelo/namedtree"
	"github.com/quorumcontrol/dgit/tupelo/repotree"
	"github.com/quorumcontrol/dgit/tupelo/usertree"
)

var log = logging.Logger("decentragit.initializer")

var validRepoName = regexp.MustCompile(`^[a-zA-Z0-9_-]+\/[a-zA-Z0-9_-]+`)

var promptTemplates = &promptui.PromptTemplates{
	Prompt:  `{{ . }} `,
	Confirm: `{{ . }} {{ "Y/n" | bold }} `,
	Valid:   `{{ . }} `,
	Invalid: `{{ . }} `,
	Success: `{{ . }} `,
}

type Options struct {
	Repo      *dgit.Repo
	Tupelo    *tupelo.Client
	NodeStore nodestore.DagStore
}

type Initializer struct {
	stdin     io.ReadCloser
	stdout    io.WriteCloser
	stderr    io.WriteCloser
	keyring   *keyring.Keyring
	auth      transport.AuthMethod
	repo      *dgit.Repo
	tupelo    *tupelo.Client
	nodestore nodestore.DagStore
}

func Init(ctx context.Context, opts *Options, args []string) error {
	i := &Initializer{
		stdin:     os.Stdin,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		repo:      opts.Repo,
		tupelo:    opts.Tupelo,
		nodestore: opts.NodeStore,
	}
	return i.Init(ctx, args)
}

func (i *Initializer) Init(ctx context.Context, args []string) error {
	var err error

	// load up auth and notify user if new
	_, err = i.getAuth(ctx)
	if err != nil {
		return err
	}

	// determine endpoint, prompt user as needed
	_, err = i.getEndpoint()
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
		"repo":    i.repo.MustName(),
		"repourl": i.repo.MustURL(),
	})
	fmt.Fprintln(os.Stdout)

	return nil
}

func (i *Initializer) findOrRequestUsername() (string, error) {
	repoConfig, err := i.repo.Config()
	if err != nil {
		return "", fmt.Errorf("could not get repo config: %w", err)
	}

	dgitConfig := repoConfig.Merged.Section(constants.DgitConfigSection)

	var username string
	if dgitConfig != nil {
		username = dgitConfig.Option("username")
	}

	// try looking up the GitHub username for the default
	if username == "" {
		if ghConfig := repoConfig.Merged.Section("github"); ghConfig != nil {
			username = ghConfig.Option("user")
		}
	}

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.Parse(msg.UsernamePrompt, nil)),
		Templates: promptTemplates,
		Default:   username,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
		Validate: func(input string) error {
			if input == "" {
				return fmt.Errorf("cannot be blank")
			}
			return nil
		},
	}
	username, err = prompt.Run()
	fmt.Fprintln(i.stdout)
	if err != nil {
		return "", fmt.Errorf("bad username: %w", err)
	}
	username = stripNewLines(username)

	globalUsername := repoConfig.Merged.GlobalConfig().Section(constants.DgitConfigSection).Option("username")

	if username != "" && username != globalUsername {
		log.Debugf("adding dgit.username to repo config")
		newConfig := repoConfig.Merged.AddOption(configformat.LocalScope, constants.DgitConfigSection, configformat.NoSubsection, "username", username)
		repoConfig.Merged.SetLocalConfig(newConfig)
		err = repoConfig.Validate()
		if err != nil {
			return "", err
		}
		err = i.repo.Storer.SetConfig(repoConfig)
		if err != nil {
			return "", err
		}
	}

	return username, nil
}

func (i *Initializer) getAuth(ctx context.Context) (transport.AuthMethod, error) {
	if i.auth != nil {
		return i.auth, nil
	}

	var err error
	i.keyring, err = keyring.NewDefault()
	// TODO: if no keyring available, prompt user for dgit password
	if err != nil {
		return nil, fmt.Errorf("Error with keyring: %v", err)
	}

	username, err := i.findOrRequestUsername()
	if err != nil {
		return nil, err
	}

	privateKey, err := i.keyring.FindPrivateKey(username)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		privateKey, err = i.createOrRecoverPrivateKey(ctx, username)
	}
	if err != nil {
		return nil, fmt.Errorf("Error fetching key from keyring: %v", err)
	}

	i.auth = dgit.NewPrivateKeyAuth(privateKey)
	return i.auth, nil
}

func (i *Initializer) createPrivateKey(ctx context.Context, username string) (*ecdsa.PrivateKey, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return nil, fmt.Errorf("error generating entropy for mnemoic seed: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, fmt.Errorf("error generating mnemoic seed: %w", err)
	}
	seed := bip39.NewSeed(mnemonic, username)

	privateKey, err := i.keyring.CreatePrivateKey(username, seed)
	if err != nil {
		return nil, fmt.Errorf("error creating private key: %w", err)
	}

	i.auth = dgit.NewPrivateKeyAuth(privateKey)

	opts := &usertree.Options{
		Name:   username,
		Tupelo: i.tupelo,
		Owners: []string{i.auth.String()},
	}

	userTree, err := usertree.Create(ctx, opts)
	if err != nil {
		i.keyring.DeletePrivateKey(username)
		return nil, err
	}

	msg.Fprint(i.stdout, msg.Welcome, map[string]interface{}{
		"username": username,
		"did":      userTree.Did(),
	})
	fmt.Fprintln(i.stdout)

	seedSlice := strings.Split(string(mnemonic), " ")

	msg.Fprint(i.stdout, msg.UserSeedPhraseCreated, map[string]interface{}{
		"username": username,
		"seed":     " " + strings.Join(seedSlice[0:8], " ") + "\n " + strings.Join(seedSlice[8:16], " ") + "\n " + strings.Join(seedSlice[16:], " "),
	})
	fmt.Fprintln(i.stdout)

	return privateKey, nil
}

func (i *Initializer) recoverPrivateKey(ctx context.Context, username string, userTree *usertree.UserTree) (*ecdsa.PrivateKey, error) {
	seedCleaner := func(str string) string {
		str = stripNewLines(str)
		str = strings.TrimSpace(str)
		return strings.ToLower(str)
	}

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.Parse(msg.PromptRecoveryPhrase, map[string]interface{}{"username": username})),
		Templates: promptTemplates,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
		Validate: func(input string) error {
			seedSlice := strings.Split(seedCleaner(input), " ")
			if len(seedSlice) != 24 {
				return fmt.Errorf(msg.PromptInvalidRecoveryPhrase)
			}
			return nil
		},
	}

	seed, err := prompt.Run()
	fmt.Fprintln(i.stdout)
	if err != nil {
		return nil, err
	}
	seed = seedCleaner(seed)

	privateKey, err := i.keyring.CreatePrivateKey(username, bip39.NewSeed(seed, username))
	if err != nil {
		return nil, fmt.Errorf("error creating private key: %w", err)
	}

	isOwner, err := userTree.IsOwner(ctx, crypto.PubkeyToAddress(privateKey.PublicKey).String())
	if err != nil {
		return nil, fmt.Errorf("error checking chaintree ownership: %w", err)
	}

	if !isOwner {
		msg.Fprint(i.stdout, msg.IncorrectRecoveryPhrase, map[string]interface{}{
			"username": username,
		})
		fmt.Fprintln(i.stdout)
		i.keyring.DeletePrivateKey(username)
		return i.recoverPrivateKey(ctx, username, userTree)
	}

	i.auth = dgit.NewPrivateKeyAuth(privateKey)

	msg.Fprint(i.stdout, msg.UserRestored, map[string]interface{}{
		"username": username,
	})
	fmt.Fprintln(i.stdout)

	return nil, nil
}

func (i *Initializer) createOrRecoverPrivateKey(ctx context.Context, username string) (*ecdsa.PrivateKey, error) {
	userTree, err := usertree.Find(ctx, username, i.tupelo)
	if errors.Is(err, usertree.ErrNotFound) {
		return i.createPrivateKey(ctx, username)
	}
	if err != nil {
		return nil, err
	}
	return i.recoverPrivateKey(ctx, username, userTree)
}

func (i *Initializer) findOrCreateRepoTree(ctx context.Context) (*repotree.RepoTree, error) {
	client, err := dgit.Default()
	if err != nil {
		return nil, err
	}

	_, err = i.getEndpoint()
	if err != nil {
		return nil, err
	}

	tree, err := client.FindRepoTree(ctx, i.repo.MustName())
	// repo already exists, return out
	if err == nil && tree != nil {
		return tree, nil
	}
	// a real error, return
	if err != namedtree.ErrNotFound {
		return nil, err
	}

	auth, err := i.getAuth(ctx)
	if err != nil {
		return nil, err
	}

	// repo doesn't exist, create it
	log.Debugf("creating new repo tree with endpoint %+v and auth %+v", i.repo.MustEndpoint(), auth)
	newTree, err := client.CreateRepoTree(ctx, i.repo.MustEndpoint(), auth)
	if errors.Is(err, usertree.ErrNotFound) {
		return nil, fmt.Errorf(msg.Parse(msg.UserNotFound, map[string]interface{}{
			"user": strings.Split(i.repo.MustName(), "/")[0],
		}))
	}
	if err != nil {
		return nil, err
	}

	msg.Fprint(i.stdout, msg.RepoCreated, map[string]interface{}{
		"did":  newTree.Did(),
		"repo": i.repo.MustName(),
	})
	fmt.Fprintln(i.stdout)

	return newTree, nil
}

func (i *Initializer) getEndpoint() (*transport.Endpoint, error) {
	rep, err := i.repo.Endpoint()

	if err == dgit.ErrEndpointNotFound {
		rep, err = i.determineDgitEndpoint()
	}

	if err != nil {
		return nil, err
	}

	if rep != nil {
		i.repo.SetEndpoint(rep)
	}

	return rep, nil
}

func (i *Initializer) determineDgitEndpoint() (*transport.Endpoint, error) {
	var dgitEndpoint *transport.Endpoint

	remotes, err := i.repo.Remotes()
	if err != nil {
		return nil, err
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

	var suggestedRepoName string
	username, _ := i.repo.Username()
	cwd, _ := os.Getwd()
	if username != "" && cwd != "" {
		suggestedRepoName = username + "/" + path.Base(cwd)
	}

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.Parse(msg.PromptRepoName, nil)),
		Templates: promptTemplates,
		Default:   suggestedRepoName,
		Stdin:     i.stdin,
		Stdout:    i.stdout,
		Validate: func(input string) error {
			input = stripNewLines(input)
			if !validRepoName.MatchString(input) {
				return fmt.Errorf(stripNewLines(msg.Parse(msg.PromptRepoNameInvalid, nil)))
			}
			return nil
		},
	}

	result, err := prompt.Run()
	fmt.Fprintln(i.stdout)
	if err != nil {
		return nil, err
	}
	result = stripNewLines(strings.ToLower(result))
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

	_, err = i.getEndpoint()
	if err != nil {
		return err
	}

	for _, url := range remoteConfig.URLs {
		if url == i.repo.MustURL() {
			// already has dgit url, no need to add another
			return nil
		}
	}

	msg.Fprint(i.stdout, msg.AddDgitToRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    i.repo.MustName(),
		"repourl": i.repo.MustURL(),
	})
	fmt.Fprintln(i.stdout)

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.Parse(msg.AddDgitToRemoteConfirm, nil)),
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

	remoteConfig.URLs = append(remoteConfig.URLs, i.repo.MustURL())

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
		"repo":    i.repo.MustName(),
		"repourl": i.repo.MustURL(),
	})
	fmt.Fprintln(i.stdout)
	return nil
}

func (i *Initializer) addDgitRemote(ctx context.Context) error {
	_, err := i.repo.Remote(constants.DgitRemote)
	if err != git.ErrRemoteNotFound {
		return err
	}

	_, err = i.getEndpoint()
	if err != nil {
		return err
	}

	remoteConfig := &config.RemoteConfig{
		Name: constants.DgitRemote,
		URLs: []string{i.repo.MustURL()},
	}
	err = remoteConfig.Validate()
	if err != nil {
		return err
	}

	msg.Fprint(i.stdout, msg.AddDgitRemote, map[string]interface{}{
		"remote":  remoteConfig.Name,
		"repo":    i.repo.MustName(),
		"repourl": i.repo.MustURL(),
	})
	fmt.Fprintln(i.stdout)

	prompt := promptui.Prompt{
		Label:     stripNewLines(msg.Parse(msg.AddDgitRemoteConfirm, nil)),
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
		"repo":    i.repo.MustName(),
		"repourl": i.repo.MustURL(),
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
