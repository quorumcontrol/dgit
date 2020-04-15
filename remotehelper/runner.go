package remotehelper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	logging "github.com/ipfs/go-log"

	"github.com/quorumcontrol/dgit/constants"
	"github.com/quorumcontrol/dgit/keyring"
	"github.com/quorumcontrol/dgit/msg"
	"github.com/quorumcontrol/dgit/transport/dgit"
)

var log = logging.Logger("dgit.runner")

type Runner struct {
	local   *git.Repository
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	keyring *keyring.Keyring
}

func New(local *git.Repository) *Runner {
	return &Runner{
		local:  local,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// > Also, what are the advantages and disadvantages of a remote helper
// > with push/fetch capabilities vs a remote helper with import/export
// > capabilities?

// It mainly has to do with what it is convenient for your helper to
// produce.  If the helper would find it more convenient to write native
// git objects (for example because the remote server speaks a
// git-specific protocol, as in the case of remote-curl.c) then the
// "fetch" capability will be more convenient.  If the helper wants to
// make a batch of new objects then a fast-import stream can be a
// convenient way to do this and the "import" capability takes care of
// running fast-import to take care of that.
//
// http://git.661346.n2.nabble.com/remote-helper-example-with-push-fetch-capabilities-td7623009.html
//

func (r *Runner) Run(ctx context.Context, remoteName string, remoteUrl string) error {
	log.Infof("running git-remote-dgit on remote %s with url %s", remoteName, remoteUrl)

	// get the named remote as reported by git, but then
	// create a new remote with only the url specified
	// this is for cases when a remote has multiple urls
	// specified for push / fetch
	namedRemote, err := r.local.Remote(remoteName)
	if err != nil {
		return err
	}

	err = namedRemote.Config().Validate()
	if err != nil {
		return fmt.Errorf("Invalid remote config: %v", err)
	}

	remote := git.NewRemote(r.local.Storer, &config.RemoteConfig{
		Name:  namedRemote.Config().Name,
		Fetch: namedRemote.Config().Fetch,
		URLs:  []string{remoteUrl},
	})

	stdinReader := bufio.NewReader(r.stdin)

	for {
		var err error

		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}
		command = strings.TrimSpace(command)
		commandParts := strings.Split(command, " ")

		log.Infof("received command on stdin %s", command)

		args := strings.TrimSpace(strings.TrimPrefix(command, commandParts[0]))
		command = commandParts[0]

		switch command {
		case "capabilities":
			r.respond(strings.Join([]string{
				"*push",
				"*fetch",
			}, "\n") + "\n")
			r.respond("\n")
		case "list":
			refs, err := remote.List(&git.ListOptions{})

			if err == transport.ErrRepositoryNotFound && args == "for-push" {
				r.respond("\n")
				continue
			}

			if err == transport.ErrRepositoryNotFound {
				return fmt.Errorf(msg.RepoNotFound)
			}

			if err == transport.ErrEmptyRemoteRepository || len(refs) == 0 {
				r.respond("\n")
				continue
			}

			if err != nil {
				return err
			}

			var head string

			listResponse := make([]string, len(refs))
			for i, ref := range refs {
				listResponse[i] = fmt.Sprintf("%s %s", ref.Hash(), ref.Name())

				// TODO: set default branch in repo chaintree which
				//       would become head here
				//
				// if master head exists, use that
				if ref.Name() == "refs/heads/master" {
					head = ref.Name().String()
				}
			}

			sort.Slice(listResponse, func(i, j int) bool {
				return strings.Split(listResponse[i], " ")[1] < strings.Split(listResponse[j], " ")[1]
			})

			// if head is empty, use last as default
			if head == "" {
				head = strings.Split(listResponse[len(listResponse)-1], " ")[1]
			}

			r.respond("@%s HEAD\n", head)
			r.respond("%s\n", strings.Join(listResponse, "\n"))
			r.respond("\n")
		case "push":
			refSpec := config.RefSpec(args)

			endpoint, err := transport.NewEndpoint(remote.Config().URLs[0])
			if err != nil {
				return err
			}

			auth, err := r.auth()
			if err != nil {
				return err
			}

			log.Debugf("auth for push: %s %s", auth.Name(), auth.String())

			err = remote.PushContext(ctx, &git.PushOptions{
				RemoteName: remote.Config().Name,
				RefSpecs:   []config.RefSpec{refSpec},
				Auth:       auth,
			})

			if err == transport.ErrRepositoryNotFound {
				err = nil // reset err back to nil
				client, err := dgit.Default()
				if err != nil {
					return err
				}

				_, err = client.CreateRepoTree(ctx, endpoint, auth)
				if err != nil {
					return err
				}

				// Retry push now that repo exists
				err = remote.PushContext(ctx, &git.PushOptions{
					RemoteName: remote.Config().Name,
					RefSpecs:   []config.RefSpec{refSpec},
					Auth:       auth,
				})
			}

			dst := refSpec.Dst(plumbing.ReferenceName("*"))
			if err != nil && err != git.NoErrAlreadyUpToDate {
				r.respond("error %s %s\n", dst, err.Error())
				break
			}

			r.respond("ok %s\n", dst)
			r.respond("\n")
		case "fetch":
			splitArgs := strings.Split(args, " ")
			if len(splitArgs) != 2 {
				return fmt.Errorf("incorrect arguments for fetch, received %s, expected 'hash refname'", args)
			}

			refName := plumbing.ReferenceName(splitArgs[1])

			refSpecs := []config.RefSpec{}

			log.Debugf("remote fetch config %v", remote.Config().Name)

			for _, fetchRefSpec := range remote.Config().Fetch {
				if !fetchRefSpec.Match(refName) {
					continue
				}

				newRefStr := ""
				if fetchRefSpec.IsForceUpdate() {
					newRefStr += "+"
				}
				newRefStr += refName.String() + ":" + fetchRefSpec.Dst(refName).String()

				newRef := config.RefSpec(newRefStr)

				if err := newRef.Validate(); err != nil {
					return err
				}

				log.Debugf("attempting to fetch on %s", newRef.String())
				refSpecs = append(refSpecs, newRef)
			}

			err := remote.FetchContext(ctx, &git.FetchOptions{
				RemoteName: remote.Config().Name,
				RefSpecs:   refSpecs,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				return err
			}
			log.Debugf("fetch complete")
			r.respond("\n")
		// Connect can be used for upload / receive pack
		// case "connect":
		// 	r.respond("fallback\n")
		case "": // command stream terminated, return out
			return nil
		default:
			return fmt.Errorf("Command '%s' not handled", command)
		}
	}
}

func (r *Runner) respond(format string, a ...interface{}) (n int, err error) {
	log.Infof("responding to git:")
	resp := bufio.NewScanner(strings.NewReader(fmt.Sprintf(format, a...)))
	for resp.Scan() {
		log.Infof("  " + resp.Text())
	}
	return fmt.Fprintf(r.stdout, format, a...)
}

func (r *Runner) userMessage(format string, a ...interface{}) (n int, err error) {
	log.Infof("responding to user:")
	resp := bufio.NewScanner(strings.NewReader(fmt.Sprintf(format, a...)))
	for resp.Scan() {
		log.Infof("  " + resp.Text())
	}
	return fmt.Fprintf(r.stderr, format+"\n", a...)
}

func (r *Runner) auth() (transport.AuthMethod, error) {
	var err error

	// mainly used for github actions
	envAuth, err := r.authFromEnv()
	if envAuth != nil || err != nil {
		return envAuth, err
	}

	if r.keyring == nil {
		r.keyring, err = keyring.NewDefault()

		// TODO: if no keyring available, prompt user for dgit password
		if err != nil {
			return nil, err
		}
	}

	repoConfig, err := r.local.Config()
	if err != nil {
		return nil, err
	}

	dgitConfig := repoConfig.Merged.Section(constants.DgitConfigSection)

	var username string
	if dgitConfig != nil {
		username = dgitConfig.Option("username")
	}

	envUsername := os.Getenv("DGIT_USERNAME")
	if envUsername != "" {
		username = envUsername
	}

	if username == "" {
		return nil, fmt.Errorf(msg.UserNotConfigured)
	}

	privateKey, err := r.keyring.FindPrivateKey(username)
	if err == keyring.ErrKeyNotFound {
		return nil, fmt.Errorf(msg.Parse(msg.PrivateKeyNotFound, map[string]interface{}{
			"keyringProvider": r.keyring.Name(),
		}))
	}

	return dgit.NewPrivateKeyAuth(privateKey), nil
}

func (r *Runner) authFromEnv() (transport.AuthMethod, error) {
	privateKeyEnv, ok := os.LookupEnv("DGIT_PRIVATE_KEY")
	if !ok {
		return nil, nil
	}

	privateKeyBytes, err := hexutil.Decode(privateKeyEnv)
	if err != nil {
		return nil, fmt.Errorf("error hex decoding DGIT_PRIVATE_KEY: %v", err)
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling DGIT_PRIVATE_KEY into ECDSA private key: %v", err)
	}

	return dgit.NewPrivateKeyAuth(privateKey), nil
}
