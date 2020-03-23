package main

import (
	"github.com/quorumcontrol/dgit/cmd"
)

var Version string

func main() {
	cmd.Version = Version
	cmd.Execute()
}
