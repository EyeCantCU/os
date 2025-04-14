package cmd

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

var (
	amiID           string
	region          string
	instanceType    string
	publicKeyPath   string
	tagName         string
	vpcID           string
	subnetID        string
	securityGroupID string
)

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch an EC2 instance",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}
		client := ec2.NewFromConfig(cfg)

		pubKey, err := ioutil.ReadFile(publicKeyPath)
		if err != nil {
			log.Fatalf("failed to read public key: %v", err)
		}
		keyName := fmt.Sprintf("key-%s", tagName)

		_, err = client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
			KeyName:           aws.String(keyName),
			PublicKeyMaterial: pubKey,
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

		if subnetID == "" {
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
			subnetID = *subnets.Subnets[0].SubnetId
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
					SubnetId:                 aws.String(subnetID),
					AssociatePublicIpAddress: aws.Bool(true),
					Groups:                   []string{securityGroupID},
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

func init() {
	launchCmd.Flags().StringVar(&amiID, "ami-id", "", "AMI ID to use (required)")
	launchCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	launchCmd.Flags().StringVar(&instanceType, "instance-type", "t3.micro", "EC2 instance type")
	launchCmd.Flags().StringVar(&publicKeyPath, "public-key", "id_ed25519.pub", "Path to SSH public key")
	launchCmd.Flags().StringVar(&tagName, "tag", "", "Tag to apply to instance and key pair (required)")
	launchCmd.Flags().StringVar(&vpcID, "vpc-id", "", "VPC ID to launch the instance into (required)")
	launchCmd.Flags().StringVar(&subnetID, "subnet-id", "", "Subnet ID to use (optional, will auto-select one from the VPC)")
	launchCmd.Flags().StringVar(&securityGroupID, "security-group-id", "", "Security group ID for the instance (required)")
	launchCmd.MarkFlagRequired("security-group-id")
	launchCmd.MarkFlagRequired("vpc-id")
	launchCmd.MarkFlagRequired("ami-id")
	launchCmd.MarkFlagRequired("region")
	launchCmd.MarkFlagRequired("tag")
}
