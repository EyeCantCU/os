package cmd

import (
	"context"
	"log"

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
	)

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into an EC2 VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, tagName)
			log.Printf("Connecting to VM at %s", publicIP)

			if err := sshutils.SSHToInstance(publicIP, privateKeyPath, sshUser, args); err != nil {
				log.Fatalf("SSH error: %v", err)
			}

		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&sshUser, "user", "ec2-user", "SSH username")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "id_ed25519", "Path to private SSH key")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("region")

	return cmd
}

// Find instance by tag
func getIPByTag(ctx context.Context, tagName string) string {
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
