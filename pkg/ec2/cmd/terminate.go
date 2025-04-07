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

var instanceID string

var terminateCmd = &cobra.Command{
	Use:   "terminate",
	Short: "Terminate EC2 instances and delete key pair by tag",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		client := ec2.NewFromConfig(cfg)

		// Describe instances with tag
		desc, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{tagName}},
				{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "stopping", "stopped"}},
			},
		})
		if err != nil {
			log.Fatalf("describe instances failed: %v", err)
		}

		var instanceIDs []string
		for _, r := range desc.Reservations {
			for _, inst := range r.Instances {
				instanceIDs = append(instanceIDs, *inst.InstanceId)
			}
		}

		if len(instanceIDs) > 0 {
			_, err = client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
				InstanceIds: instanceIDs,
			})
			if err != nil {
				log.Fatalf("terminate failed: %v", err)
			}
			log.Printf("Requested termination for instances: %v", instanceIDs)
		} else {
			log.Printf("No instances found with tag: %s", tagName)
		}

		keyName := fmt.Sprintf("key-%s", tagName)
		_, err = client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
			KeyName: &keyName,
		})
		if err != nil {
			log.Printf("failed to delete key pair: %v", err)
		} else {
			log.Printf("Deleted key pair: %s", keyName)
		}
	},
}

func init() {
	terminateCmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	terminateCmd.Flags().StringVar(&tagName, "tag", "", "Tag name to delete associated resources (required)")
	terminateCmd.MarkFlagRequired("region")
	terminateCmd.MarkFlagRequired("tag")
}
