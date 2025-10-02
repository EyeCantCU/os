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
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var (
		instanceName   string
		privateKeyPath string
		projectID      string
		sshUser        string
		zone           string
		knownHosts     string
	)

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into a GCE VM by name",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP, err := getInstanceExternalIP(ctx, projectID, zone, instanceName)
			if err != nil {
				log.Fatalf("Could not get public IP for %s: %v", instanceName, err)
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

	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVarP(&privateKeyPath, "private-key", "i", "", "Path to private SSH key")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVarP(&sshUser, "user", "u", "root", "SSH username")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")
	cmd.MarkFlagRequired("private-key")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var (
		instanceName   string
		localFilePath  string
		privateKeyPath string
		projectID      string
		sshUser        string
		zone           string
		knownHosts     string
		sudo           bool
	)

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on a GCE VM by name",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP, err := getInstanceExternalIP(ctx, projectID, zone, instanceName)
			if err != nil {
				log.Fatalf("Could not get public IP for %s: %v", instanceName, err)
			}
			log.Printf("Connecting to VM at %s", publicIP)

			remoteFile := filepath.Join("/home", sshUser, filepath.Base(localFilePath))

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

			remotecmd := append([]string{remoteFile}, args...)
			if sudo {
				remotecmd = append([]string{"sudo"}, remotecmd...)
			}

			if err := sshutils.SSHToInstance(ctx, publicIP, auth, sshUser, knownHosts, remotecmd); err != nil {
				log.Fatalf("failed to run %s on remote host: %v", filepath.Base(localFilePath), err)
			}
		},
	}

	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "", "Path to private SSH key")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&sshUser, "user", "root", "SSH username")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.Flags().BoolVar(&sudo, "sudo", false, "Run as root")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("private-key")
	cmd.MarkFlagRequired("project")

	return cmd
}

func waitForSSHCmd() *cobra.Command {
	var (
		instanceName string
		projectID    string
		zone         string
	)

	cmd := &cobra.Command{
		Use:   "wait-for-ssh",
		Short: "wait for ssh to be ready",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			var publicIP string
			var err error
			for {
				publicIP, err = getInstanceExternalIP(ctx, projectID, zone, instanceName)
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					log.Fatalf("Never found public IP for instance %s, last err: %v", instanceName, err)
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

	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")

	return cmd
}

// getInstanceExternalIP retrieves the external IP address of the created instance.
func getInstanceExternalIP(ctx context.Context, projectID, zone, instanceName string) (string, error) {
	var publicIP string
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create compute instances client: %v", err)
	}
	defer instancesClient.Close()

	req := &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: instanceName,
	}
	instance, err := instancesClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("could not get instance details: %v", err)
	}
	// Extract the external IP from the access configuration.
	for _, ni := range instance.GetNetworkInterfaces() {
		for _, ac := range ni.GetAccessConfigs() {
			if ac.NatIP != nil {
				publicIP = ac.GetNatIP() // This is IPv4 only.
			} else {
				return "", fmt.Errorf("no external IP found for instance %s", instanceName)
			}
		}
	}
	return publicIP, nil
}
