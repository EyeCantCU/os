package cmd

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils"
	awsutil "chainguard.dev/wolfi-vm/vm-test/pkg/runner/aws/util"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/spf13/cobra"
)

func consoleLogCmd() *cobra.Command {
	var (
		tagName string
		region  string
		wait    bool
	)

	cmd := &cobra.Command{
		Use:   "console-log",
		Short: "write the console log to stdout",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			client := ec2.NewFromConfig(cfg)
			if err != nil {
				log.Fatalf("failed to load AWS config: %v", err)
			}

			inst, err := awsutil.GetInstanceByTag(ctx, client, tagName)
			if err != nil {
				log.Fatalf("Failed to get instances in %s with tag %s: %v", region, tagName, err)
			}

			input := &ec2.GetConsoleOutputInput{
				InstanceId: inst.InstanceId,
				Latest:     utils.ToPtr(true),
			}

			var decoded []byte
			for {
				output, err := client.GetConsoleOutput(ctx, input)
				if err != nil {
					log.Fatalf("failed to get console output: %v", err)
				}
				if output.Output == nil {
					log.Fatalf("no console output available for instance: %s", *inst.InstanceId)
				}
				decoded, err = base64.StdEncoding.DecodeString(*output.Output)
				if err != nil {
					log.Fatalf("base64 decoding for console log of %s failed: %v", *inst.InstanceId, err)
				}
				if strings.TrimSpace(string(decoded)) != "" {
					break
				}
				if !wait {
					log.Fatalf("console output is empty for instance: %s", *inst.InstanceId)
				}
				select {
				case <-ctx.Done():
					wait = false
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}
			fmt.Println(string(decoded))
		},
	}

	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name of the instance (required)")
	cmd.Flags().StringVar(&region, "region", "", "AWS region (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for  console logs  to become available")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("region")

	return cmd
}
