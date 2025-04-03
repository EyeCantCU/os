package cmd

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Delete VPC, subnet, and security group associated with a tag",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}
		client := ec2.NewFromConfig(cfg)

		// Find and delete security groups
		sgOut, err := client.DescribeSecurityGroups(context.TODO(), &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{tagName}},
			},
		})
		if err != nil {
			log.Fatalf("failed to describe security groups: %v", err)
		}
		for _, sg := range sgOut.SecurityGroups {
			_, err := client.DeleteSecurityGroup(context.TODO(), &ec2.DeleteSecurityGroupInput{
				GroupId: sg.GroupId,
			})
			if err != nil {
				log.Printf("failed to delete security group %s: %v", *sg.GroupId, err)
			} else {
				log.Printf("Deleted security group: %s", *sg.GroupId)
			}
		}

		// Find and delete subnets
		subnetOut, err := client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{tagName}},
			},
		})
		if err != nil {
			log.Fatalf("failed to describe subnets: %v", err)
		}
		for _, subnet := range subnetOut.Subnets {
			_, err := client.DeleteSubnet(context.TODO(), &ec2.DeleteSubnetInput{
				SubnetId: subnet.SubnetId,
			})
			if err != nil {
				log.Printf("failed to delete subnet %s: %v", *subnet.SubnetId, err)
			} else {
				log.Printf("Deleted subnet: %s", *subnet.SubnetId)
			}
		}

		// Find and delete VPCs
		vpcOut, err := client.DescribeVpcs(context.TODO(), &ec2.DescribeVpcsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{tagName}},
			},
		})
		if err != nil {
			log.Fatalf("failed to describe VPCs: %v", err)
		}
		for _, vpc := range vpcOut.Vpcs {
			_, err := client.DeleteVpc(context.TODO(), &ec2.DeleteVpcInput{
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

func init() {
	teardownCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	teardownCmd.Flags().StringVar(&tagName, "tag", "", "Tag name to identify resources for deletion (required)")
	teardownCmd.MarkFlagRequired("region")
	teardownCmd.MarkFlagRequired("tag")
}
