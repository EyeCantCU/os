package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func consoleLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console-log",
		Short: "write the console log to stdout",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmdir := args[0]
			consolePath := filepath.Join(vmdir, "ttyS0.log")
			if _, err := os.Stat(consolePath); errors.Is(err, os.ErrNotExist) {
				log.Fatalf("%s does not exist.", consolePath)
			} else if err != nil {
				log.Fatalf("%s - stat failed: %v", consolePath, err)
			}

			content, err := os.ReadFile(consolePath)
			if err != nil {
				log.Fatalf("Failed to read console file %s: %v", consolePath, err)
			}

			fmt.Println(string(content))
		},
	}

	return cmd
}
