package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/windkube/aws-metrics-exporter/cmd"
)

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "dev"

func main() {
	os.Exit(run())
}

// run exists so the signal handler is torn down before os.Exit, which does not
// run deferred functions.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return cmd.Execute(ctx, version)
}
