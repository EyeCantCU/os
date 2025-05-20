package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop a vm",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = args[0]
			fmt.Printf("Dir=%s\n", dir)
			return nil
		},
	}

	return cmd
}
