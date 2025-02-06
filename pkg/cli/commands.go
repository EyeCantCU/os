package cli

import (
	"log/slog"
	"os"

	"github.com/chainguard-dev/clog/slag"
	charmlog "github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var level slag.Level
	cmd := &cobra.Command{
		Use:               "wolfi-vm",
		DisableAutoGenTag: true,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			slog.SetDefault(slog.New(charmlog.NewWithOptions(os.Stderr, charmlog.Options{ReportTimestamp: true, Level: charmlog.Level(level)})))
			return nil
		},
	}
	cmd.PersistentFlags().Var(&level, "log-level", "log level (e.g. debug, info, warn, error)")

	cmd.AddCommand(buildCmd())
	cmd.AddCommand(debugCmd())
	cmd.AddCommand(fetchCmd())
	cmd.AddCommand(makeBuilderCmd())
	return cmd
}
