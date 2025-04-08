package cmd

import (
	"context"
	"log"
	"time"

	"github.com/chainguard-dev/clog/slag"
	"github.com/spf13/cobra"
)

var (
	timeoutStr string
	cancel     context.CancelFunc
)

func New() *cobra.Command {
	var level slag.Level

	cmd := &cobra.Command{
		Use:               "qemucli",
		PersistentPreRun:  setupContext,
		PersistentPostRun: cancelContext,
	}
	cmd.PersistentFlags().Var(&level, "log-level", "log level (e.g. debug, info, warn, error)")
	cmd.PersistentFlags().StringVar(&timeoutStr, "timeout", "", "Overall operation timeout (optional)")

	cmd.AddCommand(
		createCmd(),
		startCmd(),
		sshCmd(),
		stopCmd(),
		sshInfo(),
	)

	return cmd
}

func setupContext(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			log.Fatalf("invalid timeout duration: %v", err)
		}
		ctx, cancel = context.WithTimeout(ctx, d)
	}
	cmd.SetContext(ctx)
}

func cancelContext(cmd *cobra.Command, args []string) {
	if cancel != nil {
		cancel()
	}
}

func Execute() {
	cobra.CheckErr(New().Execute())
}
