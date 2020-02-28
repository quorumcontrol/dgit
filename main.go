package main

import (
	"context"
	"fmt"
	"os"

	"github.com/quorumcontrol/decentragit-remote/runner"
	"gopkg.in/src-d/go-git.v4"
)

func main() {
	fmt.Fprintf(os.Stderr, "decentragit loaded\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	local, err := git.PlainOpen(os.Getenv("GIT_DIR"))
	if err != nil {
		panic(err)
	}

	r, err := runner.New(local)
	if err != nil {
		panic(err)
	}

	if err := r.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
