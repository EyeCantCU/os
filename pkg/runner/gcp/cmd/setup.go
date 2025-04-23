package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Nothing to do yet.")
		},
	}

	return cmd
}
