package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	repolib "github.com/quorumcontrol/decentragit-remote/repo"
)

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Usage: %s <remote-name> <url>", os.Args[0])
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := repolib.New(ctx, os.Args[1], os.Args[2])
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
		var err error

		command, err := stdinReader.ReadString('\n')
		if err != nil {
			return err
		}
		command = strings.TrimSpace(command)
		commandParts := strings.Split(command, " ")

		args := strings.TrimSpace(strings.TrimPrefix(command, commandParts[0]))
		command = commandParts[0]

		fmt.Fprintf(os.Stderr, "command=%s args=%s remote=%s url=%s\n", command, args, repo.RemoteName(), repo.Url())

		switch command {
		case "capabilities":
			fmt.Printf(strings.Join(repo.Capabilities(), "\n") + "\n")
		case "list":
			refHashList, err := repo.List()
			if err != nil {
				return err
			}
			head := ""
			for ref, hash := range refHashList {
				fmt.Printf("%s %s\n", strings.TrimSpace(hash), strings.TrimSpace(ref))
				head = ref
			}

			fmt.Printf("@%s HEAD\n", head)
		case "push":
			argsSplit := strings.Split(args, ":")
			src, dst := argsSplit[0], argsSplit[1]
			err = repo.Push(src, dst)
			if err != nil {
				return err
			}
			fmt.Printf("ok %s\n", dst)
		case "": // Final command / cleanup
			fmt.Printf("\n")
			break
		default:
			return fmt.Errorf("Command not handled")
		}

		// This ends the current command
		fmt.Printf("\n")

		if err != nil {
			fmt.Fprintf(os.Stderr, "final err: %v \n", err)
			return err
		}
	}

	return nil
}

func main() {
	fmt.Fprintf(os.Stderr, "decentragit loaded\n")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
