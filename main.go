package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"chainguard.dev/apkoaas/pkg/cli"
	"github.com/chainguard-dev/clog"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cli.New().ExecuteContext(ctx); err != nil {
		clog.Error(err.Error())
		panic(err)
	}
}
