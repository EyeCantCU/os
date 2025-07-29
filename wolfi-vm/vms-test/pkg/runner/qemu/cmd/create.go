package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"chainguard.dev/wolfi-vm/vm-test/pkg/util"
	"github.com/spf13/cobra"
)

func createCmd() *cobra.Command {
	var dir, disk, fwcode string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a vm dir",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = args[0]
			disk = args[1]

			return Create(dir, disk, fwcode)
		},
	}

	cmd.Flags().StringVar(&fwcode, "fwcode", "", "FWCODE.fd")

	cmd.MarkFlagRequired("fwcode")

	return cmd
}

func Create(dir, disk, fwcode string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("Failed to create %s: %v", dir, err)
	}

	if _, err := util.CopyFile(disk, filepath.Join(dir, "disk.raw")); err != nil {
		return fmt.Errorf("Failed copy %s ->L %s", disk, filepath.Join(dir, "disk.raw"))
	}

	if _, err := util.CopyFile(fwcode, filepath.Join(dir, "fw-code.fd")); err != nil {
		return fmt.Errorf("Failed copy %s ->L %s", disk, filepath.Join(dir, "fw-code.fd"))
	}

	fmt.Printf("Created %s\n", dir)

	return nil
}
