package cmd

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {
	var region, tagName string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Create a VPC, subnet, security group, internet gateway, and route",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				log.Fatalf("failed to load AWS config: %v", err)
			}
			client := ec2.NewFromConfig(cfg)

			// Create VPC
			vpcOut, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
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
			subnetOut, err := client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
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
			sgOut, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
				GroupName:   aws.String(fmt.Sprintf(tagName)),
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

			// Authorize SSH
			_, err = client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
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

			// Create Internet Gateway
			igwOut, err := client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
				TagSpecifications: []ec2types.TagSpecification{
					{
						ResourceType: ec2types.ResourceTypeInternetGateway,
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(tagName)},
						},
					},
				},
			})
			if err != nil {
				log.Fatalf("failed to create Internet Gateway: %v", err)
			}
			igwID := *igwOut.InternetGateway.InternetGatewayId
			log.Printf("Created Internet Gateway: %s", igwID)

			_, err = client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
				InternetGatewayId: aws.String(igwID),
				VpcId:             aws.String(vpcID),
			})
			if err != nil {
				log.Fatalf("failed to attach IGW: %v", err)
			}
			log.Printf("Attached IGW to VPC")

			// Create Route Table
			rtOut, err := client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
				VpcId: aws.String(vpcID),
				TagSpecifications: []ec2types.TagSpecification{
					{
						ResourceType: ec2types.ResourceTypeRouteTable,
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String(tagName)},
						},
					},
				},
			})
			if err != nil {
				log.Fatalf("failed to create route table: %v", err)
			}
			rtID := *rtOut.RouteTable.RouteTableId
			log.Printf("Created Route Table: %s", rtID)

			_, err = client.CreateRoute(ctx, &ec2.CreateRouteInput{
				RouteTableId:         aws.String(rtID),
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId:            aws.String(igwID),
			})
			if err != nil {
				log.Fatalf("failed to create route: %v", err)
			}

			_, err = client.CreateRoute(ctx, &ec2.CreateRouteInput{
				RouteTableId:             aws.String(rtID),
				DestinationIpv6CidrBlock: aws.String("::0/0"),
				GatewayId:                aws.String(igwID),
			})
			if err != nil {
				log.Fatalf("failed to create route: %v", err)
			}
			log.Printf("Added default route through IGW")

			_, err = client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
				SubnetId:     aws.String(subnetID),
				RouteTableId: aws.String(rtID),
			})
			if err != nil {
				log.Fatalf("failed to associate route table: %v", err)
			}
			log.Printf("Associated route table with subnet")

			log.Println("Setup complete.")
		},
	}

	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name to apply to resources (required)")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("tag")

	return cmd
}
