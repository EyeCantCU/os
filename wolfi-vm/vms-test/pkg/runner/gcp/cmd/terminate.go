package cmd

import (
	"log"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/cobra"
)

func terminateCmd() *cobra.Command {
	var (
		instanceName string
		projectID    string
		zone         string
	)

	cmd := &cobra.Command{
		Use:   "terminate",
		Short: "Terminate GCE VMs by name",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			instancesClient, err := compute.NewInstancesRESTClient(ctx)
			if err != nil {
				log.Fatalf("Failed to create compute instances client: %v", err)
			}
			defer instancesClient.Close()

			req := &computepb.DeleteInstanceRequest{
				Instance: instanceName,
				Project:  projectID,
				Zone:     zone,
			}
			op, err := instancesClient.Delete(ctx, req)
			if err != nil {
				log.Fatalf("could not delete instance: %v", err)
			}
			if err := op.Wait(ctx); err != nil {
				log.Fatalf("instance deletion operation failed: %v", err)
			}
			log.Printf("Instance %s deleted successfully.", instanceName)
		},
	}

	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")

	return cmd
}
