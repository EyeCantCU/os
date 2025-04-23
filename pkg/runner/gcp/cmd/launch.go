package cmd

import (
	"fmt"
	"log"
	"os"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

func launchCmd() *cobra.Command {
	var bootDiskSizeGb int64
	var bootDiskType string
	var instanceName string
	var labelName string
	var machineType string
	var projectID string
	var publicKeyPath string
	var serviceAccount string
	var sourceImageUri string
	var sshUser string
	var vpcNetwork string
	var zone string

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a GCE VM",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			pubKey, err := os.ReadFile(publicKeyPath)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			// Create the Compute Engine Instances client.
			instancesClient, err := compute.NewInstancesRESTClient(ctx)
			if err != nil {
				log.Fatalf("Failed to create compute instances client: %v", err)
			}
			defer instancesClient.Close()

			// Populate the instance protobuf.
			instance := &computepb.Instance{
				Name: proto.String(instanceName),
				Disks: []*computepb.AttachedDisk{
					{
						Boot:       proto.Bool(true),
						AutoDelete: proto.Bool(true), // Note that when the instance is deleted, the boot disk is also deleted.
						Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
						InitializeParams: &computepb.AttachedDiskInitializeParams{
							DiskSizeGb:  proto.Int64(bootDiskSizeGb),
							DiskType:    proto.String(fmt.Sprintf("zones/%s/diskTypes/%s", zone, bootDiskType)),
							SourceImage: proto.String(sourceImageUri),
						},
					},
				},
				Labels:      map[string]string{"test": labelName}, // TODO we need to decide on what the label key:value should be.
				MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType)),
				Metadata: &computepb.Metadata{
					Items: []*computepb.Items{
						{
							Key:   proto.String("ssh-keys"),
							Value: proto.String(fmt.Sprintf("%s:%s", sshUser, pubKey)),
						},
					},
				},
				NetworkInterfaces: []*computepb.NetworkInterface{
					{
						Network: proto.String(fmt.Sprintf("projects/%s/global/networks/%s", projectID, vpcNetwork)),
						AccessConfigs: []*computepb.AccessConfig{
							{
								Name: proto.String("External NAT"),
								Type: proto.String("ONE_TO_ONE_NAT"),
							},
						},
					},
				},
				ServiceAccounts: []*computepb.ServiceAccount{
					{
						Email: proto.String(serviceAccount), // We use the GCE default service account unless specified.
						Scopes: []string{
							"https://www.googleapis.com/auth/cloud-platform",
						},
					},
				},
			}

			req := &computepb.InsertInstanceRequest{
				Project:          projectID,
				Zone:             zone,
				InstanceResource: instance,
			}
			op, err := instancesClient.Insert(ctx, req)
			if err != nil {
				log.Print("Maybe try gcloud auth login --update-adc") // Try to handle this in setup instead.
				log.Fatalf("could not create instance: %v", err)
			}
			// Wait until the operation completes.
			if err := op.Wait(ctx); err != nil {
				log.Fatalf("instance creation operation failed: %v", err)
			}
			log.Printf("Launched GCE VM: %s", instanceName)
		},
	}

	cmd.Flags().Int64Var(&bootDiskSizeGb, "disk-size", 10, "Size of the boot disk in Gb")
	cmd.Flags().StringVar(&bootDiskType, "disk-type", "pd-balanced", "GCE disk type to use")
	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&labelName, "label", "", "Label to apply to resources (required)")
	cmd.Flags().StringVar(&machineType, "machine-type", "e2-standard-2", "GCE machine type")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&publicKeyPath, "public-key", "", "Path to SSH public key")
	cmd.Flags().StringVar(&serviceAccount, "service-account", "default", "Service account to attach to the VM (optional)")
	cmd.Flags().StringVar(&sourceImageUri, "source-image-uri", "", "Image or image family URI (required)")
	cmd.Flags().StringVar(&sshUser, "ssh-user", "root", "Username for SSH")
	cmd.Flags().StringVar(&vpcNetwork, "vpc-network", "default", "VPC network to use")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")

	cmd.MarkFlagRequired("image-uri")
	cmd.MarkFlagRequired("label")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")
	cmd.MarkFlagRequired("public-key")

	return cmd
}
