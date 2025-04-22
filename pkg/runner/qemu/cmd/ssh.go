package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var vmdir, sshKeyFile, user string
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

			if err := sshutils.SSHToInstance(ctx,
				fmt.Sprintf("%s:%d", sshAddr, sshPort), sshKeyFile, user, sshArgs); err != nil {
				log.Fatalf("SSH error: %v", err)
			}

		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "linky", "USER")
	cmd.Flags().StringVarP(&sshKeyFile, "private-key", "i", "", "id_ed25519")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var vmdir, sshKeyFile, user, localFilePath string

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on an Azure VM by tag",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			vmdir = args[0]
			cmdArgs := args[1:]

			sshAddr, sshPort, err := getSSHPortAddr(ctx, filepath.Join(vmdir, "qmp.sock"))
			if err != nil {
				log.Fatalf("Failed to get ssh port from %s: %v\n", vmdir, err)
			}
			hostPort := fmt.Sprintf("%s:%d", sshAddr, sshPort)

			remoteFile := filepath.Join("/tmp", filepath.Base(localFilePath))

			err = sshutils.ShoveBinaryFile(ctx, hostPort, sshKeyFile, user, localFilePath, remoteFile)
			if err != nil {
				log.Fatalf("failed to move %s to remote host: %v", localFilePath, err)
			}

			remotecmd := append([]string{"exec", remoteFile}, cmdArgs...)

			if err := sshutils.SSHToInstance(ctx, hostPort, sshKeyFile, user, remotecmd); err != nil {
				log.Fatalf("failed to run %s on remote host: %v", filepath.Base(localFilePath), err)
			}
		},
	}

	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVarP(&user, "user", "u", "linky", "USER")
	cmd.Flags().StringVarP(&sshKeyFile, "private-key", "i", "", "id_ed25519")
	cmd.MarkFlagRequired("file")

	return cmd
}

func waitForSSHCmd() *cobra.Command {
	var vmdir string

	cmd := &cobra.Command{
		Use:   "wait-for-ssh",
		Short: "wait for ssh to be ready",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			vmdir = args[0]

			sshAddr, sshPort, err := getSSHPortAddr(ctx, filepath.Join(vmdir, "qmp.sock"))
			if err != nil {
				log.Fatalf("Failed to get ssh port from %s: %v", vmdir, err)
			}
			hostPort := fmt.Sprintf("%s:%d", sshAddr, sshPort)

			log.Printf("Waiting for hostkey from %s", hostPort)
			key, err := sshutils.WaitForSSHHostKey(ctx, hostPort, time.Duration(2*time.Minute), time.Duration(200*time.Millisecond))

			if err != nil {
				log.Fatalf("Wait for ssh hostkey failed: %v", key)
			}

			fmt.Printf("%s %s %s\n", hostPort, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
		},
	}

	return cmd
}
