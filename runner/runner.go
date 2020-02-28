package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/quorumcontrol/decentragit-remote/client"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var log = logging.Logger("dgit.runner")

var defaultLogLevel = "PANIC"

type Runner struct {
	local  *git.Repository
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func New(local *git.Repository) (*Runner, error) {
	return &Runner{
		local:  local,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

func (r *Runner) respond(format string, a ...interface{}) (n int, err error) {
	log.Debug("responding to git:")
	log.Debug(format, a)
	return fmt.Fprintf(r.stdout, format, a...)
}

func (r *Runner) SetLogLevel() {
	logLevelStr, ok := os.LookupEnv("DGIT_LOG_LEVEL")
	if !ok {
		logLevelStr = defaultLogLevel
	}

	err := logging.SetLogLevelRegex("dgit.*", strings.ToUpper(logLevelStr))
	if err != nil {
		fmt.Fprintf(r.stderr, "invalid value %s given for DGIT_LOG_LEVEL: %v", logLevelStr, err)
	}
}

func (r *Runner) Run(ctx context.Context, args []string) error {
	r.SetLogLevel()
	log.Debugf("running %v", strings.Join(args, " "))

	if len(args) < 3 {
		return fmt.Errorf("Usage: %s <remote-name> <url>", args[0])
	}

	client, err := client.New(ctx)
	if err != nil {
		return err
	}
	client.RegisterAsDefault()

	remoteName := args[1]
	remote, err := r.local.Remote(remoteName)
	if err != nil {
		return err
	}

	stdinReader := bufio.NewReader(r.stdin)

	tty, err := os.Create("/dev/tty")
	if err != nil {
		return err
	}

	ttyReader := bufio.NewReader(tty)

	if ttyReader == nil {
		return fmt.Errorf("ttyReader is nil")
	}

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
				"list",
				"push",
				"fetch",
			}, "\n") + "\n")
		case "list":
			refs, err := remote.List(&git.ListOptions{})
			if err != nil {
				return err
			}

			for _, ref := range refs {
				r.respond("%s %s\n", ref.Name(), ref.Hash())
			}

		// 	r.respond("@%s HEAD\n", head)
		case "push":
			refSpec := config.RefSpec(args)

			err := remote.PushContext(ctx, &git.PushOptions{
				RemoteName: remote.Config().Name,
				RefSpecs:   []config.RefSpec{refSpec},
			})
			if err != nil {
				panic(err)
			}
			dst := refSpec.Dst(plumbing.ReferenceName("*"))

			r.respond("ok %s\n", dst)
		case "": // Final command / cleanup
			r.respond("\n")
			break
		default:
			return fmt.Errorf("Command not handled")
		}

		// This ends the current command
		r.respond("\n")

		if err != nil {
			return err
		}
	}

	return nil
}
