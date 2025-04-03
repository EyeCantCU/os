package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create a VPC, subnet, and security group for EC2 instances",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}
		client := ec2.NewFromConfig(cfg)

		// Create VPC
		vpcOut, err := client.CreateVpc(context.TODO(), &ec2.CreateVpcInput{
			CidrBlock: aws.String("10.0.0.0/24"),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeVpc,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(tagName)},
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("failed to create VPC: %v", err)
		}
		vpcID := *vpcOut.Vpc.VpcId
		log.Printf("Created VPC: %s", vpcID)

		// Create Subnet
		subnetOut, err := client.CreateSubnet(context.TODO(), &ec2.CreateSubnetInput{
			VpcId:     aws.String(vpcID),
			CidrBlock: aws.String("10.0.0.0/24"),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSubnet,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(tagName)},
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("failed to create subnet: %v", err)
		}
		subnetID := *subnetOut.Subnet.SubnetId
		log.Printf("Created Subnet: %s", subnetID)

		// Create Security Group
		sgOut, err := client.CreateSecurityGroup(context.TODO(), &ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(fmt.Sprintf("%s-ssh", tagName)),
			Description: aws.String("SSH access and all egress"),
			VpcId:       aws.String(vpcID),
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: ec2types.ResourceTypeSecurityGroup,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(tagName)},
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("failed to create security group: %v", err)
		}
		sgID := *sgOut.GroupId
		log.Printf("Created Security Group: %s", sgID)

		// Authorize SSH Inbound (IPv4 and IPv6)
		_, err = client.AuthorizeSecurityGroupIngress(context.TODO(), &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(sgID),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(22),
					ToPort:     aws.Int32(22),
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges: []ec2types.Ipv6Range{
						{CidrIpv6: aws.String("::/0")},
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("failed to authorize ingress: %v", err)
		}

		/*
			// Allow all egress
			_, err = client.AuthorizeSecurityGroupEgress(context.TODO(), &ec2.AuthorizeSecurityGroupEgressInput{
				GroupId: aws.String(sgID),
				IpPermissions: []ec2types.IpPermission{
					{
						IpProtocol: aws.String("-1"),
						IpRanges: []ec2types.IpRange{
							{CidrIp: aws.String("0.0.0.0/0")},
						},
						Ipv6Ranges: []ec2types.Ipv6Range{
							{CidrIpv6: aws.String("::/0")},
						},
					},
				},
			})
			if err != nil {
				log.Fatalf("failed to authorize egress: %v", err)
			}
		*/

		log.Println("Setup complete.")
	},
}

func init() {
	setupCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	setupCmd.Flags().StringVar(&tagName, "tag", "", "Tag name to apply to all created resources (required)")
	setupCmd.MarkFlagRequired("region")
	setupCmd.MarkFlagRequired("tag")
}
