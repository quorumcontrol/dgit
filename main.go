package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	repolib "github.com/quorumcontrol/decentragit-remote/repo"
)

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: %s <remote-name> <url>", os.Args[0])
	}

	repo, err := repolib.New(os.Args[1], os.Args[2])
	if err != nil {
		return err
	}

	stdinReader := bufio.NewReader(os.Stdin)

	tty, err := os.Create("/dev/tty")
	if err != nil {
		return err
	}

	ttyReader := bufio.NewReader(tty)

	if ttyReader == nil {
		return fmt.Errorf("ttyReader is nil")
	}

	for {
		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}
		command = strings.TrimSpace(command)

		fmt.Fprintf(os.Stderr, "command=%s remote=%s url=%s\n", command, repo.RemoteName(), repo.Url())

		switch command {
		case "capabilities":
			fmt.Printf(strings.Join(repo.Capabilities(), "\n"))
			fmt.Printf("\n\n")
		case "list":
			fmt.Printf("\n")
		case "list for-push":
			fmt.Printf("\n")
		case "push":
			fmt.Printf("\n")
		case "export":
			fmt.Printf("\n")
		case "":
			break
		default:
			return fmt.Errorf("Command not handled")
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
