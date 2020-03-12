package cmd

import (
	"fmt"
	"os"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
)

const defaultLogLevel = "PANIC"

var log = logging.Logger("dgit.cmd")

var rootCmd = &cobra.Command{
	Use:   "dgit",
	Short: "dgit is git with decentralized ownership and storage",
	Long:  `This is the dgit CLI, useful for initializing dgit in repos.`,
}

func Execute() {
	setLogLevel()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setLogLevel() {
	// turn off all logging, mainly for silencing tupelo-go-sdk ERROR logs
	logging.SetAllLoggers(logging.LevelPanic)

	// now set dgit.* logs if applicable
	logLevelStr, ok := os.LookupEnv("DGIT_LOG_LEVEL")
	if !ok {
		logLevelStr = defaultLogLevel
	}

	err := logging.SetLogLevelRegex("dgit.*", strings.ToUpper(logLevelStr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid value %s given for DGIT_LOG_LEVEL: %v\n", logLevelStr, err)
	}
}
