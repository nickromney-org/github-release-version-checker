package main

import (
	"os"

	"github.com/nickromney-org/github-release-version-checker/cmd"
)

// Version information (set via ldflags during build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Pass version info to cmd package
	cmd.SetVersionInfo(Version, BuildTime, GitCommit)

	if err := cmd.Execute(); err != nil {
		cmd.PrintError(err)
		os.Exit(cmd.ExitCode(err))
	}
}
