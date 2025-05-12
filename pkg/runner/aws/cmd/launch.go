package cmd

import (
	"fmt"
	"log"
	"os"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/aws/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

func launchCmd() *cobra.Command {
	var (
		amiID           string
		region          string
		instanceType    string
		publicKeyPath   string
		tagName         string
		launchGroupName string
		vpcID           string
		subnetID        string
		securityGroupID string
	)

	launchGroup := util.LaunchGroup{}

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch an EC2 instance",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				log.Fatalf("failed to load AWS config: %v", err)
			}
			client := ec2.NewFromConfig(cfg)

			if launchGroupName != "" {
				launchGroup, err = util.GetLaunchGroupByTag(ctx, client, launchGroupName)
				if err != nil {
					log.Fatalf("Error retrieving launch-group '%s': %v", launchGroupName, err)
				}
			} else if vpcID != "" {
				launchGroup.SecurityGroupID = securityGroupID
				launchGroup.VpcID = vpcID
				launchGroup.SubnetID = subnetID
			} else if val := os.Getenv("VMT_LAUNCH_GROUP"); val != "" {
				launchGroup, err = util.GetLaunchGroupByTag(ctx, client, val)
			} else {
				log.Fatalf("Must provide launch group info via --launch-group, (--security-group and --vpc-id), or environment VMT_LAUNCH_GROUP")
			}

			pubKey, err := sshutils.GetSinglePublicKey(publicKeyPath)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			keyName := fmt.Sprintf("key-%s", tagName)

			_, err = client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
				KeyName:           aws.String(keyName),
				PublicKeyMaterial: []byte(pubKey),
				TagSpecifications: []ec2types.TagSpecification{
					{
						ResourceType: ec2types.ResourceTypeKeyPair,
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(tagName)},
						},
					},
				},
			})
			if err != nil {
				log.Fatalf("failed to import key pair: %v", err)
			}

			if launchGroup.SubnetID == "" {
				subnets, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
					Filters: []ec2types.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []string{vpcID},
						},
					},
				})
				if err != nil {
					log.Fatalf("failed to describe subnets in VPC %s: %v", vpcID, err)
				}
				if len(subnets.Subnets) == 0 {
					log.Fatalf("no subnets found in VPC %s", vpcID)
				}
				launchGroup.SubnetID = *subnets.Subnets[0].SubnetId
				log.Printf("Selected subnet %s from VPC %s", subnetID, vpcID)
			}

			runOut, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
				ImageId:      aws.String(amiID),
				InstanceType: ec2types.InstanceType(instanceType),
				MinCount:     aws.Int32(1),
				MaxCount:     aws.Int32(1),
				KeyName:      aws.String(keyName),
				TagSpecifications: []ec2types.TagSpecification{
					{
						ResourceType: ec2types.ResourceTypeInstance,
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(tagName)},
						},
					},
				},
				NetworkInterfaces: []ec2types.InstanceNetworkInterfaceSpecification{
					{
						DeviceIndex:              aws.Int32(0),
						SubnetId:                 aws.String(launchGroup.SubnetID),
						AssociatePublicIpAddress: aws.Bool(true),
						Groups:                   []string{launchGroup.SecurityGroupID},
					},
				},
			})
			if err != nil {
				log.Fatalf("failed to launch instance: %v", err)
			}

			instanceID := *runOut.Instances[0].InstanceId
			log.Printf("Launched instance ID: %s", instanceID)
		},
	}

	cmd.Flags().StringVar(&amiID, "ami-id", "", "AMI ID to use (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&instanceType, "instance-type", "t3.micro", "EC2 instance type")
	cmd.Flags().StringVar(&publicKeyPath, "public-key", "id_ed25519.pub", "Path to SSH public key")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag to apply to instance and key pair (required)")
	cmd.Flags().StringVar(&vpcID, "vpc-id", "", "VPC ID to launch the instance into (required)")
	cmd.Flags().StringVar(&subnetID, "subnet-id", "", "Subnet ID to use (optional, will auto-select one from the VPC)")
	cmd.Flags().StringVar(&securityGroupID, "security-group-id", "", "Security group ID for the instance")
	cmd.Flags().StringVar(&launchGroupName, "launch-group", "", "the tag created to setup")
	cmd.MarkFlagsRequiredTogether("vpc-id", "security-group-id")
	cmd.MarkFlagsMutuallyExclusive("launch-group", "vpc-id")
	cmd.MarkFlagRequired("ami-id")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("tag")

	return cmd
}
