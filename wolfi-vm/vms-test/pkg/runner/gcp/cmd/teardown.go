package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func teardownCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "teardown",
		Short: "Teardown ephermal resources",
		Run: func(cmd *cobra.Command, args []string) {
			// Currently, there is nothing to tear down.
			log.Printf("Nothing to do yet.")
		},
	}

	return cmd
}
