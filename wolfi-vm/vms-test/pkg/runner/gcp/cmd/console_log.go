package cmd

import (
	"fmt"
	"log"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func consoleLogCmd() *cobra.Command {
	var (
		instanceName string
		projectID    string
		zone         string
	)

	cmd := &cobra.Command{
		Use:   "console-log",
		Short: "write the console log to stdout",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			instancesClient, err := compute.NewInstancesRESTClient(ctx)
			if err != nil {
				log.Fatalf("failed to create compute instances client: %v", err)
			}
			defer instancesClient.Close()
			req := &computepb.GetSerialPortOutputInstanceRequest{
				Instance: instanceName,
				Project:  projectID,
				Zone:     zone,
				Port:     proto.Int32(1),
			}
			resp, err := instancesClient.GetSerialPortOutput(ctx, req)
			if err != nil {
				log.Fatalf("could not get serial port output: %v", err)
			}
			content := resp.GetContents()
			fmt.Println(string(content))
		},
	}

	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")

	return cmd
}
