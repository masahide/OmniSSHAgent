package cli

import (
	"flag"
	"fmt"
	"io"
	"runtime"

	"github.com/masahide/OmniSSHAgent/internal/config"
)

const (
	ExitOK             = 0
	ExitInternal       = 1
	ExitUsage          = 2
	ExitConfiguration  = 3
	ExitAlreadyRunning = 4
	ExitBackend        = 5
	ExitInterface      = 6
	ExitWindows        = 7
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Result struct {
	StartApplication bool
	Code             int
}

func Run(args []string, stdout, stderr io.Writer) Result {
	if len(args) == 0 {
		return Result{StartApplication: true}
	}
	if args[0] == "--version" || args[0] == "version" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "version accepts no arguments")
			return Result{Code: ExitUsage}
		}
		fmt.Fprintf(stdout, "Version: %s\nCommit: %s\nBuild time: %s\nGOOS: %s\nGOARCH: %s\n", Version, Commit, BuildTime, runtime.GOOS, runtime.GOARCH)
		return Result{}
	}
	switch args[0] {
	case "config-path":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "config-path accepts no arguments")
			return Result{Code: ExitUsage}
		}
		path, err := config.DefaultConfigPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return Result{Code: ExitInternal}
		}
		fmt.Fprintln(stdout, path)
		return Result{}
	case "check-config":
		fs := flag.NewFlagSet("check-config", flag.ContinueOnError)
		fs.SetOutput(stderr)
		defaultPath, err := config.DefaultConfigPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return Result{Code: ExitInternal}
		}
		path := fs.String("config", defaultPath, "configuration file path")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
			if err == nil {
				fmt.Fprintln(stderr, "unexpected arguments")
			}
			return Result{Code: ExitUsage}
		}
		if _, err := config.LoadRuntime(*path); err != nil {
			fmt.Fprintln(stderr, err)
			return Result{Code: ExitConfiguration}
		}
		fmt.Fprintf(stdout, "configuration is valid: %s\n", *path)
		return Result{}
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return Result{Code: ExitUsage}
	}
}
