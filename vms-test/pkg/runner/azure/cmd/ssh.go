package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/azure/azutil"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var (
		resourceGroup  string
		vmTag          string
		subscriptionID string
		privateKeyPath string
		sshUser        string
		knownHosts     string
	)

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into an Azure VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to get Azure credentials: %v", err)
			}
			publicIP, err := azutil.GetIPByTag(ctx, cred, subscriptionID, resourceGroup, vmTag)
			if err != nil {
				log.Fatalf("Could not get public IP for %s: %v", vmTag, err)
			}
			log.Printf("Connecting to VM at %s", publicIP)

			if knownHosts == "" {
				var err error
				knownHosts, err = sshutils.EphemeralKnownHosts()
				if err != nil {
					log.Printf("could not get ephemeral known hosts file: %v, using /dev/null", err)
					knownHosts = "/dev/null"
				}
				defer os.Remove(knownHosts)
			}

			if len(args) == 0 {
				sshutils.SSHCommand(ctx, publicIP, sshUser, privateKeyPath, knownHosts, args)
				return
			}

			auth, err := sshutils.GetPrivateKeyOrDefaultAuth(privateKeyPath)
			if err != nil {
				log.Fatalf("Error setting up auth: %v", err)
			}

			if err := sshutils.SSHToInstance(ctx, publicIP, auth, sshUser, knownHosts, args); err != nil {
				log.Fatalf("SSH error: %v", err)
			}

		},
	}

	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&vmTag, "tag", "", "Tag name of the VM (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVarP(&sshUser, "user", "u", "azureuser", "SSH username")
	cmd.Flags().StringVarP(&privateKeyPath, "private-key", "i", "id_ed25519", "Path to private SSH key")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("subscription")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var (
		resourceGroup  string
		vmTag          string
		subscriptionID string
		privateKeyPath string
		sshUser        string
		localFilePath  string
		knownHosts     string
	)

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on an Azure VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to get Azure credentials: %v", err)
			}
			publicIP, err := azutil.GetIPByTag(ctx, cred, subscriptionID, resourceGroup, vmTag)
			if err != nil {
				log.Fatalf("Could not get public IP for %s: %v", vmTag, err)
			}
			log.Printf("Connecting to VM at %s", publicIP)

			remoteFile := filepath.Join("/tmp", filepath.Base(localFilePath))

			if knownHosts == "" {
				var err error
				knownHosts, err = sshutils.EphemeralKnownHosts()
				if err != nil {
					log.Printf("could not get ephemeral known hosts file: %v, using /dev/null", err)
					knownHosts = "/dev/null"
				}
				defer os.Remove(knownHosts)
			}

			auth, err := sshutils.GetPrivateKeyOrDefaultAuth(privateKeyPath)
			if err != nil {
				log.Fatalf("Error setting up auth: %v", err)
			}

			err = sshutils.ShoveBinaryFile(ctx, publicIP, auth, sshUser, knownHosts, localFilePath, remoteFile)
			if err != nil {
				log.Fatalf("failed to move %s to remote host: %v", localFilePath, err)
			}

			remotecmd := append([]string{"exec", remoteFile}, args...)

			if err := sshutils.SSHToInstance(ctx, publicIP, auth, sshUser, knownHosts, remotecmd); err != nil {
				log.Fatalf("failed to run %s on remote host: %v", filepath.Base(localFilePath), err)
			}
		},
	}

	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&vmTag, "tag", "", "Tag name of the VM (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVarP(&sshUser, "user", "u", "azureuser", "SSH username")
	cmd.Flags().StringVarP(&privateKeyPath, "private-key", "i", "id_ed25519", "Path to private SSH key")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("subscription")

	return cmd
}

func waitForSSHCmd() *cobra.Command {
	var (
		resourceGroup  string
		vmTag          string
		subscriptionID string
	)

	cmd := &cobra.Command{
		Use:   "wait-for-ssh",
		Short: "wait for ssh to be ready",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to get Azure credentials: %v", err)
			}
			var publicIP string
			for {
				publicIP, err = azutil.GetIPByTag(ctx, cred, subscriptionID, resourceGroup, vmTag)
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					log.Fatalf("Never found public IP for tag %s, last err: %v", vmTag, err)
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}

			log.Printf("Waiting for hostkey from %s", publicIP)
			key, err := sshutils.WaitForSSHHostKey(ctx, publicIP, time.Duration(500*time.Millisecond))

			if err != nil {
				log.Fatalf("Wait for hostkey from %s failed: %v", publicIP, err)
			}

			fmt.Printf("%s %s %s\n", publicIP, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))
		},
	}

	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&vmTag, "tag", "", "Tag name of the VM (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("subscription")

	return cmd
}
