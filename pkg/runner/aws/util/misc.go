package util

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func getInstanceByTag(ctx context.Context, client ec2.Client, tagName string) ([]ec2types.Instance, error) {
	instances := []ec2types.Instance{}

	out, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:Name"), Values: []string{tagName}},
		},
	})
	if err != nil {
		return instances, err
	}

	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			instances = append(instances, inst)
		}
	}

	return instances, nil
}
