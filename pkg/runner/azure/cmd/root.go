package cmd

import (
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"
)

var (
	timeoutStr string
	cancel     context.CancelFunc
)

var rootCmd = &cobra.Command{
	Use:               "azurecli",
	Short:             "CLI tool to manage Azure instances",
	PersistentPreRun:  setupContext,
	PersistentPostRun: cancelContext,
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
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&timeoutStr, "timeout", "", "Overall operation timeout (optional)")
	rootCmd.AddCommand(setupCmd())
	rootCmd.AddCommand(launchCmd())
	rootCmd.AddCommand(sshCmd())
	rootCmd.AddCommand(runRemoteCmd())
	rootCmd.AddCommand(waitForSSHCmd())
	rootCmd.AddCommand(terminateCmd())
	rootCmd.AddCommand(teardownCmd())
}
