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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var (
		privateKeyPath string
		sshUser        string
		tagName        string
		region         string
		knownHosts     string
	)

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into an EC2 VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, region, tagName)
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

	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVarP(&sshUser, "user", "u", "ec2-user", "SSH username")
	cmd.Flags().StringVarP(&privateKeyPath, "private-key", "i", "id_ed25519", "Path to private SSH key")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("region")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var (
		privateKeyPath string
		sshUser        string
		localFilePath  string
		tagName        string
		region         string
		knownHosts     string
	)

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on an EC2 VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, region, tagName)
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

	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVarP(&sshUser, "user", "u", "ec2-user", "SSH username")
	cmd.Flags().StringVarP(&privateKeyPath, "private-key", "i", "id_ed25519", "Path to private SSH key")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known hosts file")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("region")

	return cmd
}

// Find instance by tag
func getIPByTag(ctx context.Context, region string, tagName string) string {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:Name"), Values: []string{tagName}},
			{Name: aws.String("instance-state-name"), Values: []string{"running"}},
		},
	})
	if err != nil {
		log.Fatalf("describe failed: %v", err)
	}
	var publicIP string
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			if inst.PublicIpAddress != nil {
				publicIP = *inst.PublicIpAddress
				break
			}
		}
	}
	if publicIP == "" {
		log.Fatalf("No running instance found with tag %s", tagName)
	}
	return publicIP
}

func waitForSSHCmd() *cobra.Command {
	var (
		tagName string
		region  string
	)

	cmd := &cobra.Command{
		Use:   "wait-for-ssh",
		Short: "wait for ssh to be ready",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, region, tagName)
			log.Printf("Waiting for ssh VM at %s", publicIP)
			key, err := sshutils.WaitForSSHHostKey(ctx, publicIP, time.Duration(500*time.Millisecond))
			if err != nil {
				log.Fatalf("Wait for hostkey from %s failed: %v", publicIP, err)
			}
			fmt.Printf("%s %s %s\n", publicIP, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))

		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("region")

	return cmd
}
