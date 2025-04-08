package cmd

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var vmdir, sshKeyFile string
	cmd := &cobra.Command{
		Use:   "ssh vmdir",
		Short: "ssh to a running vm in dir",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			vmdir = args[0]
			sshArgs := args[1:]
			sshAddr, sshPort, err := getSSHPortAddr(ctx, filepath.Join(vmdir, "qmp.sock"))
			if err != nil {
				log.Fatalf("Failed to get ssh port from %s: %v\n", vmdir, err)
			}

			log.Printf("connecting to %s:%d", sshAddr, sshPort)
			if err := sshutils.SSHToInstance(ctx,
				fmt.Sprintf("%s:%d", sshAddr, sshPort), sshKeyFile, "backdoor", sshArgs); err != nil {
				log.Fatalf("SSH error: %v", err)
			}

		},
	}

	cmd.Flags().StringVarP(&sshKeyFile, "private-key", "i", "", "id_ed25519")

	return cmd
}
