package cmd

import (
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

func teardownCmd() *cobra.Command {
	var region, tagName string

	cmd := &cobra.Command{
		Use:   "teardown",
		Short: "Delete VPC, subnet, and security group associated with a tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				log.Fatalf("failed to load AWS config: %v", err)
			}
			client := ec2.NewFromConfig(cfg)

			// Detach and delete Internet Gateways
			igwOut, err := client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:Name"), Values: []string{tagName}},
				},
			})
			if err != nil {
				log.Fatalf("failed to describe IGWs: %v", err)
			}
			for _, igw := range igwOut.InternetGateways {
				for _, attachment := range igw.Attachments {
					if attachment.VpcId != nil {
						_, err = client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
							InternetGatewayId: igw.InternetGatewayId,
							VpcId:             attachment.VpcId,
						})
						if err != nil {
							log.Printf("failed to detach IGW %s from VPC %s: %v", *igw.InternetGatewayId, *attachment.VpcId, err)
						} else {
							log.Printf("Detached IGW %s from VPC %s", *igw.InternetGatewayId, *attachment.VpcId)
						}
					}
				}
				_, err := client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
					InternetGatewayId: igw.InternetGatewayId,
				})
				if err != nil {
					log.Printf("failed to delete IGW %s: %v", *igw.InternetGatewayId, err)
				} else {
					log.Printf("Deleted IGW: %s", *igw.InternetGatewayId)
				}
			}

			// Delete Route Tables
			rtOut, err := client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:Name"), Values: []string{tagName}},
				},
			})
			if err != nil {
				log.Fatalf("failed to describe route tables: %v", err)
			}
			for _, rt := range rtOut.RouteTables {
				for _, route := range rt.Routes {
					if route.GatewayId != nil && *route.GatewayId != "local" && route.DestinationCidrBlock != nil {
						_, err = client.DeleteRoute(ctx, &ec2.DeleteRouteInput{
							RouteTableId:         rt.RouteTableId,
							DestinationCidrBlock: route.DestinationCidrBlock,
						})
						if err != nil {
							log.Printf("failed to delete route %s in route table %s: %v", *route.DestinationCidrBlock, *rt.RouteTableId, err)
						} else {
							log.Printf("Deleted route %s in route table %s", *route.DestinationCidrBlock, *rt.RouteTableId)
						}
					}
				}
				_, err = client.DeleteRouteTable(ctx, &ec2.DeleteRouteTableInput{
					RouteTableId: rt.RouteTableId,
				})
				if err != nil {
					log.Printf("failed to delete route table %s: %v", *rt.RouteTableId, err)
				} else {
					log.Printf("Deleted route table: %s", *rt.RouteTableId)
				}
			}

			// Find and delete security groups
			sgOut, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:Name"), Values: []string{tagName}},
				},
			})
			if err != nil {
				log.Fatalf("failed to describe security groups: %v", err)
			}
			for _, sg := range sgOut.SecurityGroups {
				_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
					GroupId: sg.GroupId,
				})
				if err != nil {
					log.Printf("failed to delete security group %s: %v", *sg.GroupId, err)
				} else {
					log.Printf("Deleted security group: %s", *sg.GroupId)
				}
			}

			// Find and delete subnets
			subnetOut, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:Name"), Values: []string{tagName}},
				},
			})
			if err != nil {
				log.Fatalf("failed to describe subnets: %v", err)
			}
			for _, subnet := range subnetOut.Subnets {
				_, err := client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
					SubnetId: subnet.SubnetId,
				})
				if err != nil {
					log.Printf("failed to delete subnet %s: %v", *subnet.SubnetId, err)
				} else {
					log.Printf("Deleted subnet: %s", *subnet.SubnetId)
				}
			}

			// Find and delete VPCs
			vpcOut, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("tag:Name"), Values: []string{tagName}},
				},
			})
			if err != nil {
				log.Fatalf("failed to describe VPCs: %v", err)
			}
			for _, vpc := range vpcOut.Vpcs {
				_, err := client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
					VpcId: vpc.VpcId,
				})
				if err != nil {
					log.Printf("failed to delete VPC %s: %v", *vpc.VpcId, err)
				} else {
					log.Printf("Deleted VPC: %s", *vpc.VpcId)
				}
			}

			log.Println("Teardown complete.")
		},
	}

	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name to identify resources for deletion (required)")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("tag")

	return cmd
}
