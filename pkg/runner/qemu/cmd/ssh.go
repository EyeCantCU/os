package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var vmdir, sshKeyFile, user, knownHosts string
	cmd := &cobra.Command{
		Use:   "ssh vmdir",
		Short: "ssh to a running vm in dir",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			if knownHosts == "" {
				var err error
				knownHosts, err = sshutils.EphemeralKnownHosts()
				if err != nil {
					log.Printf("could not get ephemeral known hosts file: %v, using /dev/null", err)
					knownHosts = "/dev/null"
				}
				defer os.Remove(knownHosts)
			}

			vmdir = args[0]
			sshArgs := args[1:]

			sshAddr, sshPort, err := getSSHPortAddr(ctx, filepath.Join(vmdir, "qmp.sock"))
			if err != nil {
				log.Fatalf("Failed to get ssh port from %s: %v\n", vmdir, err)
			}
			addr := fmt.Sprintf("%s:%d", sshAddr, sshPort)

			if len(sshArgs) == 0 {
				sshutils.SSHCommand(ctx, addr, user, sshKeyFile, knownHosts, sshArgs)
				return
			}

			auth, err := sshutils.GetPrivateKeyOrDefaultAuth(sshKeyFile)
			if err != nil {
				log.Fatalf("Error setting up auth: %v", err)
			}

			if err := sshutils.SSHToInstance(ctx, addr, auth, user, knownHosts, sshArgs); err != nil {
				log.Fatalf("SSH error: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "linky", "USER")
	cmd.Flags().StringVarP(&sshKeyFile, "private-key", "i", "", "id_ed25519")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var vmdir, sshKeyFile, user, knownHosts, localFilePath string

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on an Azure VM by tag",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			vmdir = args[0]
			cmdArgs := args[1:]

			if knownHosts == "" {
				var err error
				knownHosts, err = sshutils.EphemeralKnownHosts()
				if err != nil {
					log.Printf("could not get ephemeral known hosts file: %v, using /dev/null", err)
					knownHosts = "/dev/null"
				}
				defer os.Remove(knownHosts)
			}

			sshAddr, sshPort, err := getSSHPortAddr(ctx, filepath.Join(vmdir, "qmp.sock"))
			if err != nil {
				log.Fatalf("Failed to get ssh port from %s: %v\n", vmdir, err)
			}
			hostPort := fmt.Sprintf("%s:%d", sshAddr, sshPort)

			remoteFile := filepath.Join("/tmp", filepath.Base(localFilePath))

			auth, err := sshutils.GetPrivateKeyOrDefaultAuth(sshKeyFile)
			if err != nil {
				log.Fatalf("Error setting up auth: %v", err)
			}

			err = sshutils.ShoveBinaryFile(ctx, hostPort, auth, user, knownHosts, localFilePath, remoteFile)
			if err != nil {
				log.Fatalf("failed to move %s to remote host: %v", localFilePath, err)
			}

			remotecmd := append([]string{"exec", remoteFile}, cmdArgs...)

			if err := sshutils.SSHToInstance(ctx, hostPort, auth, user, knownHosts, remotecmd); err != nil {
				log.Fatalf("failed to run %s on remote host: %v", filepath.Base(localFilePath), err)
			}
		},
	}

	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVarP(&user, "user", "u", "linky", "USER")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
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
			key, err := sshutils.WaitForSSHHostKey(ctx, hostPort, time.Duration(500*time.Millisecond))

			if err != nil {
				log.Fatalf("Wait for hostkey from %s failed: %v", hostPort, err)
			}

			fmt.Printf("%s %s %s\n", hostPort, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
		},
	}

	return cmd
}
