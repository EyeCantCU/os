package cmd

import (
	"fmt"
	"log"

	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/aws/util"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
)

var instanceID string

func terminateCmd() *cobra.Command {
	var (
		region  string
		tagName string
	)

	cmd := &cobra.Command{
		Use:   "terminate",
		Short: "Terminate EC2 instances and delete key pair by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				log.Fatalf("failed to load config: %v", err)
			}
			client := ec2.NewFromConfig(cfg)

			instances, err := util.GetInstancesByTag(ctx, client, tagName)

			var instanceIDs []string
			for _, inst := range instances {
				if inst.State.Name == types.InstanceStateNameTerminated {
					log.Printf("skipping instance %s [already terminated]", *inst.InstanceId)
					continue
				}
				instanceIDs = append(instanceIDs, *inst.InstanceId)
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

	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name to delete associated resources (required)")
	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("tag")

	return cmd
}
