package remotehelper

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	fixtures "github.com/go-git/go-git-fixtures/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
	logging "github.com/ipfs/go-log"
	"github.com/stretchr/testify/require"
	"github.com/tyler-smith/go-bip39"

	"github.com/quorumcontrol/dgit/keyring"
	"github.com/quorumcontrol/dgit/transport/dgit"
	"github.com/quorumcontrol/dgit/tupelo/usertree"
)

func TestRunnerIntegration(t *testing.T) {
	defer fixtures.Clean()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := os.Setenv("DG_OBJ_STORAGE", "chaintree")
	require.Nil(t, err)

	// Just generating a random username
	key, err := crypto.GenerateKey()
	require.Nil(t, err)
	username := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).String()[20:])

	err = os.Setenv("DG_USERNAME", username)
	require.Nil(t, err)

	client, err := dgit.NewLocalClient(ctx)
	require.Nil(t, err)
	client.RegisterAsDefault()

	logLevelStr, ok := os.LookupEnv("DG_LOG_LEVEL")
	if ok {
		require.Nil(t, logging.SetLogLevelRegex("decentragit.*", strings.ToUpper(logLevelStr)))
	}

	localRepoFs := fixtures.Basic().One().DotGit()
	store := filesystem.NewStorageWithOptions(localRepoFs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})

	local, err := git.Open(store, nil)
	require.Nil(t, err)

	// Just a random dg url
	endpoint, err := transport.NewEndpoint("dg://" + username + "/test")
	require.Nil(t, err)

	remoteConfig := &config.RemoteConfig{
		Name: "dgit-test",
		URLs: []string{endpoint.String()},
	}
	require.Nil(t, remoteConfig.Validate())

	_, err = local.CreateRemote(remoteConfig)
	require.Nil(t, err)

	gitInputWriter := newBlockingReader()
	gitOutpuReaderPipe, gitOutputWriter := io.Pipe()
	gitOutputReader := newTestOutputReader(gitOutpuReaderPipe)
	userMsgReaderPipe, userMsgWriter := io.Pipe()
	userMsgReader := newTestOutputReader(userMsgReaderPipe)
	require.NotNil(t, userMsgReader)

	kr := keyring.NewMemory()
	seed := bip39.NewSeed("123", username)
	pkey, err := kr.CreatePrivateKey(username, seed)
	require.Nil(t, err)
	auth := dgit.NewPrivateKeyAuth(pkey)

	_, err = usertree.Create(ctx, &usertree.Options{
		Name:   username,
		Tupelo: client.Tupelo,
		Owners: []string{auth.String()},
	})
	require.Nil(t, err)

	runner := &Runner{
		local:   local,
		stdin:   gitInputWriter,
		stdout:  gitOutputWriter,
		stderr:  userMsgWriter,
		keyring: kr,
	}

	go func() {
		err = runner.Run(ctx, remoteConfig.Name, remoteConfig.URLs[0])
		require.Nil(t, err)
	}()

	t.Run("it can list capabilities", func(t *testing.T) {
		_, err = gitInputWriter.Write([]byte("capabilities\n"))
		require.Nil(t, err)

		gitOutputReader.Expect(t, "*push\n")
		gitOutputReader.Expect(t, "*fetch\n")
		gitOutputReader.Expect(t, "\n")
	})

	t.Run("it can push a branch with same source name", func(t *testing.T) {
		_, err = gitInputWriter.Write([]byte("list for-push\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "\n")

		_, err = gitInputWriter.Write([]byte("push refs/heads/master:refs/heads/master\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "ok refs/heads/master\n")
		gitOutputReader.Expect(t, "\n")
	})

	t.Run("it can push a branch with different source name", func(t *testing.T) {
		_, err = gitInputWriter.Write([]byte("list for-push\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "@refs/heads/master HEAD\n")
		gitOutputReader.Expect(t, "6ecf0ef2c2dffb796033e5a02219af86ec6584e5 refs/heads/master\n")
		gitOutputReader.Expect(t, "\n")

		_, err = gitInputWriter.Write([]byte("push refs/heads/master:refs/heads/feature/test\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "ok refs/heads/feature/test\n")
		gitOutputReader.Expect(t, "\n")
	})

	t.Run("it can delete a branch", func(t *testing.T) {
		_, err = gitInputWriter.Write([]byte("list for-push\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "@refs/heads/master HEAD\n")
		gitOutputReader.Expect(t, "6ecf0ef2c2dffb796033e5a02219af86ec6584e5 refs/heads/feature/test\n")
		gitOutputReader.Expect(t, "6ecf0ef2c2dffb796033e5a02219af86ec6584e5 refs/heads/master\n")
		gitOutputReader.Expect(t, "\n")

		_, err = gitInputWriter.Write([]byte("push :refs/heads/feature/test\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "ok refs/heads/feature/test\n")
		gitOutputReader.Expect(t, "\n")

		_, err = gitInputWriter.Write([]byte("list\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "@refs/heads/master HEAD\n")
		gitOutputReader.Expect(t, "6ecf0ef2c2dffb796033e5a02219af86ec6584e5 refs/heads/master\n")
		gitOutputReader.Expect(t, "\n")
	})

	t.Run("it can pull a new branch", func(t *testing.T) {
		// create a second repo with different commits
		secondRepoFs := fixtures.Basic()[2].DotGit()
		secondStore := filesystem.NewStorageWithOptions(secondRepoFs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})

		secondRepo, err := git.Open(secondStore, nil)
		require.Nil(t, err)

		_, err = secondRepo.CreateRemote(remoteConfig)
		require.Nil(t, err)

		auth, err := runner.auth()
		require.Nil(t, err)

		// now manually push other commits to repo tree
		err = secondRepo.Push(&git.PushOptions{
			RemoteName: remoteConfig.Name,
			RefSpecs:   []config.RefSpec{config.RefSpec("refs/heads/master:refs/heads/second-repo-master")},
			Auth:       auth,
		})
		require.Nil(t, err)

		// now check that runner can pull new tree
		_, err = gitInputWriter.Write([]byte("list\n"))
		require.Nil(t, err)
		gitOutputReader.Expect(t, "@refs/heads/master HEAD\n")
		gitOutputReader.Expect(t, "6ecf0ef2c2dffb796033e5a02219af86ec6584e5 refs/heads/master\n")
		gitOutputReader.Expect(t, "1980fcf55330d9d94c34abee5ab734afecf96aba refs/heads/second-repo-master\n")
	})
}

type testOutputReader struct {
	*bufio.Reader
}

func newTestOutputReader(rd io.Reader) *testOutputReader {
	return &testOutputReader{bufio.NewReader(rd)}
}

func (r *testOutputReader) Expect(t *testing.T, value string) {
	line, err := r.ReadString('\n')
	require.Nil(t, err)
	require.Equal(t, value, line)
}

type blockingReader struct {
	buf    bytes.Buffer
	cond   *sync.Cond
	closed bool
}

func newBlockingReader() *blockingReader {
	m := sync.Mutex{}
	return &blockingReader{
		cond:   sync.NewCond(&m),
		buf:    bytes.Buffer{},
		closed: false,
	}
}

func (br *blockingReader) Write(b []byte) (ln int, err error) {
	ln, err = br.buf.Write(b)
	br.cond.Broadcast()
	return
}

func (br *blockingReader) Read(b []byte) (ln int, err error) {
	if br.closed {
		return ln, io.EOF
	}

	ln, err = br.buf.Read(b)
	if err == io.EOF {
		br.cond.L.Lock()
		br.cond.Wait()
		br.cond.L.Unlock()
		ln, err = br.buf.Read(b)
	}
	return
}

func (br *blockingReader) Close() {
	br.closed = true
}
