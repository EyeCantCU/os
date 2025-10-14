package main

import (
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifacts",
		Short: "Manage and extract test artifacts",
		Long:  `A tool for managing test artifacts including metrics and files from VM tests.`,
	}

	cmd.AddCommand(
		extractCmd(),
	)

	return cmd
}

func Execute() {
	cobra.CheckErr(New().Execute())
}
