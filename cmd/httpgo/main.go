package main

import (
	"context"
	"os"
	"os/signal"

	"capimgrosso.com/httpgo/internal/cli"
	"capimgrosso.com/httpgo/internal/transport"
)

// Link-time build metadata, set with -ldflags -X; never changed at runtime.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	command := cli.NewRootCommand(transport.NewNetHTTPExecutor(), cli.BuildInfo{Version: version, Commit: commit, Date: date})
	command.SetIn(os.Stdin)
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	command.SetArgs(os.Args[1:])
	return command.ExecuteContext(ctx)
}
