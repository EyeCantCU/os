package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"chainguard.dev/wolfi-vm/vms-test/pkg/internal/utils/sshutils"
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

const (
	LAUNCH_ATTEMPTS_MAX = 5
	LAUNCH_BACKOFF_SECS = 10
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
	var minCPUPlatform string
	var nestedVirt bool
	var zone string
	var extraMetadata string
	var userDataPath string
	var secondaryDiskSizeGb int64
	var secondaryDiskType string
	var secondaryDiskName string

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a GCE VM",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			pubKey, err := sshutils.GetSinglePublicKey(publicKeyPath)
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
			disks := []*computepb.AttachedDisk{
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
			}

			// Add secondary disk if specified
			if secondaryDiskSizeGb > 0 {
				// Use the instance name for the disk name so it is unique
				diskName := fmt.Sprintf("%s-data", instanceName)
				// Allow setting the in guest device name with the provided flag. This translates to /dev/disk/by-id/google-[deviceName]
				deviceName := secondaryDiskName
				if secondaryDiskName == "" {
					deviceName = fmt.Sprintf("%s-data", instanceName)
				}
				// Use boot disk type if secondary disk type not specified
				diskType := secondaryDiskType
				if secondaryDiskType == "" {
					diskType = bootDiskType
				}
				secondaryDisk := &computepb.AttachedDisk{
					Boot:       proto.Bool(false),
					AutoDelete: proto.Bool(true),
					DeviceName: proto.String(deviceName),
					Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						DiskSizeGb: proto.Int64(secondaryDiskSizeGb),
						DiskType:   proto.String(fmt.Sprintf("zones/%s/diskTypes/%s", zone, diskType)),
						DiskName:   proto.String(diskName),
					},
				}
				disks = append(disks, secondaryDisk)
			}

			instance := &computepb.Instance{
				Name:        proto.String(instanceName),
				Disks:       disks,
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
				ShieldedInstanceConfig: &computepb.ShieldedInstanceConfig{
					EnableSecureBoot: proto.Bool(true),
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
			if nestedVirt {
				if instance.AdvancedMachineFeatures == nil {
					instance.AdvancedMachineFeatures = &computepb.AdvancedMachineFeatures{}
				}
				instance.AdvancedMachineFeatures.EnableNestedVirtualization = proto.Bool(true)
			}
			if minCPUPlatform != "" {
				instance.MinCpuPlatform = &minCPUPlatform
			}

			metadata := parseMetadataString(extraMetadata)
			for k, v := range metadata {
				instance.Metadata.Items = append(instance.Metadata.Items, &computepb.Items{
					Key:   &k,
					Value: &v,
				})
			}

			if userDataPath != "" {
				if _, exists := metadata["user-data"]; exists {
					log.Fatalf("cannot attach user data file as meta data with key 'user-data' has already been specified explicitly")
				}
				userData, err := os.ReadFile(userDataPath)
				if err != nil {
					log.Fatalf("failed to read user data file: %v", err)
				}
				instance.Metadata.Items = append(instance.Metadata.Items, &computepb.Items{
					Key:   proto.String("user-data"),
					Value: proto.String(string(userData)),
				})
			}

			req := &computepb.InsertInstanceRequest{
				Project:          projectID,
				Zone:             zone,
				InstanceResource: instance,
			}

			for attempt := 1; attempt <= LAUNCH_ATTEMPTS_MAX; attempt++ {
				op, err := instancesClient.Insert(ctx, req)
				if err != nil {
					log.Print("Maybe try gcloud auth login --update-adc") // Try to handle this in setup instead.
					log.Fatalf("could not create instance: %v", err)
				}

				err = op.Wait(ctx)
				if err == nil {
					break
				}

				if isResourceExhaustionError(err) {
					log.Printf("Zone %s has insufficient capacity (attempt %d/%d failed): %v", zone, attempt, LAUNCH_ATTEMPTS_MAX, err)
					if attempt < LAUNCH_ATTEMPTS_MAX {
						backoffSecs := attempt * LAUNCH_BACKOFF_SECS
						time.Sleep(time.Duration(backoffSecs) * time.Second)
						continue
					}
				}

				log.Fatalf("instance creation operation failed: %v", err)
			}
			log.Printf("Launched GCE VM: %s", instanceName)
		},
	}

	cmd.Flags().Int64Var(&bootDiskSizeGb, "disk-size", 10, "Size of the boot disk in Gb")
	cmd.Flags().StringVar(&bootDiskType, "disk-type", "pd-balanced", "GCE disk type to use")
	cmd.Flags().StringVar(&instanceName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&labelName, "label", "", "Label to apply to resources (required)")
	cmd.Flags().StringVar(&machineType, "machine-type", "n2-standard-2", "GCE machine type")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&publicKeyPath, "public-key", "", "Path to SSH public key")
	cmd.Flags().StringVar(&serviceAccount, "service-account", "default", "Service account to attach to the VM (optional)")
	cmd.Flags().StringVar(&sourceImageUri, "source-image-uri", "", "Image or image family URI (required)")
	cmd.Flags().StringVar(&sshUser, "ssh-user", "root", "Username for SSH")
	cmd.Flags().StringVar(&vpcNetwork, "vpc-network", "default", "VPC network to use")
	cmd.Flags().StringVar(&zone, "zone", "us-central1-a", "GCE zone for resources (required)")
	cmd.Flags().StringVar(&minCPUPlatform, "min-cpu-platform", "", "Minimum CPU Platform")
	cmd.Flags().StringVar(&extraMetadata, "metadata", "", "Extra metadata for the instance. Syntax is same as gcloud (key=value;key2=val2)")
	cmd.Flags().StringVar(&userDataPath, "user-data", "", "Path to user-data file")
	cmd.Flags().BoolVar(&nestedVirt, "nested-virt", false, "Enable nested virtualization")
	cmd.Flags().Int64Var(&secondaryDiskSizeGb, "secondary-disk-size", 0, "Size of secondary disk in Gb (0 = no secondary disk)")
	cmd.Flags().StringVar(&secondaryDiskType, "secondary-disk-type", "", "GCE disk type for secondary disk (defaults to same as disk-type)")
	cmd.Flags().StringVar(&secondaryDiskName, "secondary-disk-name", "", "Device name for secondary disk (defaults to {instance-name}-data)")

	cmd.MarkFlagRequired("image-uri")
	cmd.MarkFlagRequired("label")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("project")

	return cmd
}

func parseMetadataString(input string) map[string]string {
	res := make(map[string]string)
	for _, item := range strings.Split(input, ";") {
		if item == "" {
			continue
		}
		split := strings.SplitN(item, "=", 2)
		if len(split) != 2 {
			log.Fatalf("malformed metadata %q can't be parsed into key value pair", item)
		}
		res[split[0]] = split[1]
	}
	return res
}

func isResourceExhaustionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ZONE_RESOURCE_POOL_EXHAUSTED")
}
