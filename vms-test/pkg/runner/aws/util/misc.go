package util

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func GetInstanceByTag(ctx context.Context, client *ec2.Client, tagName string) (ec2types.Instance, error) {
	instances, err := GetInstancesByTag(ctx, client, tagName)
	if err != nil {
		return ec2types.Instance{}, err
	} else if len(instances) != 1 {
		return ec2types.Instance{}, fmt.Errorf("Found %d instances with tag in %s in %s",
			len(instances), tagName, client.Options().Region)
	}

	return instances[0], nil
}

func GetInstancesByTag(ctx context.Context, client *ec2.Client, tagName string) ([]ec2types.Instance, error) {
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
