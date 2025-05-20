package util

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type LaunchGroup struct {
	SecurityGroupID string `yaml:"security-group"`
	SubnetID        string `yaml:"subnet-id"`
	VpcID           string `yaml:"vpc-id"`
}

func GetLaunchGroupByTag(ctx context.Context, client *ec2.Client, tag string) (LaunchGroup, error) {
	var lgroup LaunchGroup

	tagFilter := types.Filter{
		Name:   aws.String("tag:Name"),
		Values: []string{tag},
	}

	// 1. Describe Security Groups
	sgOut, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{tagFilter},
	})
	if err != nil {
		return lgroup, fmt.Errorf("error describing security groups: %w", err)
	}
	if len(sgOut.SecurityGroups) == 0 {
		return lgroup, fmt.Errorf("no security group found with tag %q", tag)
	} else if len(sgOut.SecurityGroups) > 1 {
		return lgroup, fmt.Errorf("no found %d security groups with tag %q",
			len(sgOut.SecurityGroups), tag)
	}
	lgroup.SecurityGroupID = aws.ToString(sgOut.SecurityGroups[0].GroupId)

	// 2. Describe Subnets
	subnetOut, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{tagFilter},
	})
	if err != nil {
		return lgroup, fmt.Errorf("error describing subnets: %w", err)
	}
	if len(subnetOut.Subnets) == 0 {
		return lgroup, fmt.Errorf("no subnet found with tag %q", tag)
	}
	lgroup.SubnetID = aws.ToString(subnetOut.Subnets[0].SubnetId)

	// 3. Describe VPCs
	vpcOut, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{tagFilter},
	})
	if err != nil {
		return lgroup, fmt.Errorf("error describing VPCs: %w", err)
	}
	if len(vpcOut.Vpcs) == 0 {
		return lgroup, fmt.Errorf("no VPC found with tag %q", tag)
	}
	lgroup.VpcID = aws.ToString(vpcOut.Vpcs[0].VpcId)

	return lgroup, nil
}
