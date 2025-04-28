package cmd

import (
	"context"
	"log"
	"os"
	"path/filepath"

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
			publicIP := getInstanceExternalIP(ctx, projectID, zone, instanceName)
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

			if err := sshutils.SSHToInstance(ctx, publicIP, privateKeyPath, sshUser, knownHosts, args); err != nil {
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
	)

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on a GCE VM by name",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getInstanceExternalIP(ctx, projectID, zone, instanceName)
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

			err := sshutils.ShoveBinaryFile(ctx, publicIP, privateKeyPath, sshUser, knownHosts, localFilePath, remoteFile)
			if err != nil {
				log.Fatalf("failed to move %s to remote host: %v", localFilePath, err)
			}

			remotecmd := append([]string{"exec", remoteFile}, args...)

			if err := sshutils.SSHToInstance(ctx, publicIP, privateKeyPath, sshUser, knownHosts, remotecmd); err != nil {
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
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("private-key")
	cmd.MarkFlagRequired("project")

	return cmd
}

// getInstanceExternalIP retrieves the external IP address of the created instance.
func getInstanceExternalIP(ctx context.Context, projectID, zone, instanceName string) string {
	var publicIP string
	instancesClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create compute instances client: %v", err)
	}
	defer instancesClient.Close()

	req := &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: instanceName,
	}
	instance, err := instancesClient.Get(ctx, req)
	if err != nil {
		log.Fatalf("could not get instance details: %v", err)
	}
	// Extract the external IP from the access configuration.
	for _, ni := range instance.GetNetworkInterfaces() {
		for _, ac := range ni.GetAccessConfigs() {
			if ac.NatIP != nil {
				publicIP = ac.GetNatIP() // This is IPv4 only.
			}
		}
	}
	if err != nil {
		log.Fatalf("no external IP found for instance %s", instanceName)
	}
	return publicIP
}
