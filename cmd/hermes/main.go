package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"hermes-tunnel/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	info := cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	if err := cli.Execute(ctx, info); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
