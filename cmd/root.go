package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	logging "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
)

const defaultLogLevel = "PANIC"

var log = logging.Logger("decentragit.cmd")

var rootCmd = &cobra.Command{
	Use:   "git gd [command]",
	Short: "decentragit is git with decentralized ownership and storage",
}

func Execute() {
	setLogLevel()
	globalDebugLogs()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func globalDebugLogs() {
	log.Infof("decentragit version: " + Version)
	log.Infof("goos: " + runtime.GOOS)
	log.Infof("goarch: " + runtime.GOARCH)
}

func setLogLevel() {
	// turn off all logging, mainly for silencing tupelo-go-sdk ERROR logs
	logging.SetAllLoggers(logging.LevelPanic)

	// now set decentragit.* logs if applicable
	logLevelStr, ok := os.LookupEnv("DG_LOG_LEVEL")
	if !ok {
		logLevelStr, ok = os.LookupEnv("DGIT_LOG_LEVEL")
		if ok {
			log.Warningf("[DEPRECATION] - DGIT_LOG_LEVEL is deprecated, please use DG_LOG_LEVEL")
		}
	}

	if logLevelStr == "" {
		logLevelStr = defaultLogLevel
	}

	err := logging.SetLogLevelRegex("decentragit.*", strings.ToUpper(logLevelStr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid value %s given for DG_LOG_LEVEL: %v\n", logLevelStr, err)
	}
}
