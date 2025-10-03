package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"chainguard.dev/wolfi-vm/vm-test/pkg/util"
	"github.com/spf13/cobra"
)

func createCmd() *cobra.Command {
	var dir, disk, fwcode, fwvars string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a vm dir",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = args[0]
			disk = args[1]

			return Create(dir, disk, fwcode, fwvars)
		},
	}

	cmd.Flags().StringVar(&fwcode, "fwcode", "", "FWCODE.fd")
	cmd.Flags().StringVar(&fwvars, "fwvars", "", "FWVARS.fd")

	cmd.MarkFlagRequired("fwcode")
	cmd.MarkFlagRequired("fwvars")

	return cmd
}

func Create(dir, disk, fwcode, fwvars string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("Failed to create %s: %v", dir, err)
	}

	if _, err := util.CopyFile(disk, filepath.Join(dir, "disk.raw")); err != nil {
		return fmt.Errorf("Failed copy %s ->L %s", disk, filepath.Join(dir, "disk.raw"))
	}

	if _, err := util.CopyFile(fwcode, filepath.Join(dir, "fw-code.fd")); err != nil {
		return fmt.Errorf("Failed copy %s ->L %s", disk, filepath.Join(dir, "fw-code.fd"))
	}

	if _, err := util.CopyFile(fwvars, filepath.Join(dir, "fw-vars.fd")); err != nil {
		return fmt.Errorf("Failed copy %s ->L %s", disk, filepath.Join(dir, "fw-vars.fd"))
	}

	fmt.Printf("Created %s\n", dir)

	return nil
}
