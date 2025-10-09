package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/spf13/cobra"
)

var (
	verbose        bool
	name           string
	arch           string
	resourceGroup  string
	gallery        string
	imageVersion   string
	definitionName string
	makeDefinition bool
	existingDisk   bool
	diskName       string
	uploadOnly     bool
	regions        []string
	tags           map[string]string
	subscriptionID string
	uploadJobs     int
	diskSizeGB     int
)

const (
	defaultResourceGroup = "chainguard-vms"
	defaultGallery       = "vmtesting"
	defaultPublisher     = "TestingPublisher"
	defaultOffer         = "TestingOffer"
	defaultVersion       = "0.0.1"
	defaultRegions       = "eastus,westus2"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "azure-image-upload [flags] image-file",
		Short: "Upload raw disk image to Azure and create gallery image",
		Long: `Upload raw disk image to Azure using Azure SDK for Go.

This tool replaces the azure-image-upload bash script with a native Go implementation
using the Azure SDK instead of az CLI commands.

Examples:
  # Upload a disk image to Azure gallery
  azure-image-upload --name my-image --arch x64 ./disk.raw

  # Upload with custom resource group and gallery
  azure-image-upload --name my-image --resource-group my-rg --gallery my-gallery ./disk.raw

  # Upload only the disk without creating gallery image
  azure-image-upload --name my-image --upload-only ./disk.raw

  # Create new image definition and version
  azure-image-upload --name my-image --make-definition --image-version 1.0.0 ./disk.raw`,
		Args: cobra.ExactArgs(1),
		RunE: runUpload,
	}

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().StringVar(&name, "name", "", "Image name (default: derived from filename)")
	rootCmd.Flags().StringVar(&arch, "arch", "", "Architecture (x64, arm64) (default: auto-detect)")
	rootCmd.Flags().StringVar(&subscriptionID, "subscription-id", "ad60e736-b0ce-432e-b77c-4b8218f452ae", "Azure subscription ID")
	rootCmd.Flags().StringVar(&resourceGroup, "resource-group", defaultResourceGroup, "Azure resource group")
	rootCmd.Flags().StringVar(&gallery, "gallery", defaultGallery, "Azure compute gallery")
	rootCmd.Flags().StringVar(&imageVersion, "image-version", defaultVersion, "Image version")
	rootCmd.Flags().StringVar(&definitionName, "definition-name", "", "Image definition name (default: same as name)")
	rootCmd.Flags().BoolVar(&makeDefinition, "make-definition", true, "Create image definition if it doesn't exist")
	rootCmd.Flags().BoolVar(&existingDisk, "existing-disk", false, "Use existing disk instead of creating new one")
	rootCmd.Flags().StringVar(&diskName, "disk-name", "", "Disk name (default: same as name)")
	rootCmd.Flags().BoolVar(&uploadOnly, "upload-only", false, "Only upload disk, don't create gallery image")
	rootCmd.Flags().StringSliceVar(&regions, "regions", strings.Split(defaultRegions, ","), "Target regions")
	rootCmd.Flags().StringToStringVar(&tags, "tags", map[string]string{"env": "dev"}, "Resource tags (key=value)")
	rootCmd.Flags().IntVarP(&uploadJobs, "jobs", "j", runtime.NumCPU(), "Jobs for uploading disk (default: num CPUs)")
	rootCmd.Flags().IntVar(&diskSizeGB, "disk-size", 30, "Size to expand VHD to in GB")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runUpload(cmd *cobra.Command, args []string) error {
	imagePath := args[0]

	if _, err := os.Stat(imagePath); !existingDisk && err != nil {
		return fmt.Errorf("image file not found: %w", err)
	}

	if name == "" {
		name = strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
		name = strings.TrimSuffix(name, ".vhd")
	}

	if diskName == "" {
		diskName = name
	}

	if definitionName == "" {
		definitionName = name
	}

	if arch == "" {
		arch = runtime.GOARCH
		log.Printf("Auto-detected architecture: %s", arch)
	}

	azArch, err := archEnum(arch)
	if err != nil {
		return err
	}

	if len(regions) < 1 {
		return fmt.Errorf("specify at least one region")
	}

	if uploadJobs < 1 {
		return fmt.Errorf("must have at least one upload job")
	}

	if diskSizeGB < 1 {
		return fmt.Errorf("must have at least 1GB disk size")
	}

	azTags := make(map[string]*string)
	for k, v := range tags {
		azTags[k] = &v
	}

	if verbose {
		log.Printf("Uploading image: %s", imagePath)
		log.Printf("Subscription: %s", subscriptionID)
		log.Printf("Name: %s", name)
		log.Printf("Architecture: %s", arch)
		log.Printf("Resource group: %s", resourceGroup)
		log.Printf("Gallery: %s", gallery)
		log.Printf("Disk name: %s", diskName)
		log.Printf("Regions: %v", regions)
		log.Printf("Tags: %v", tags)
	}

	ctx := context.Background()

	clients, err := createAzureClients(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create Azure clients: %w", err)
	}

	var diskID string

	if !existingDisk {
		diskID, err = uploadDiskImage(ctx, clients, imagePath, diskName, diskSizeGB, resourceGroup, regions[0], azArch, azTags, uploadJobs, verbose)
		if err != nil {
			return fmt.Errorf("failed to upload disk image: %w", err)
		}

		if uploadOnly {
			log.Printf("Disk uploaded successfully: %s", diskID)
			return nil
		}
	} else {
		diskID, err = getExistingDiskID(ctx, clients, diskName, resourceGroup)
		if err != nil {
			return fmt.Errorf("failed to get existing disk ID: %w", err)
		}
	}

	if makeDefinition {
		err = createImageDefinition(ctx, clients, gallery, definitionName, resourceGroup, regions[0], azArch, azTags, verbose)
		if err != nil {
			return fmt.Errorf("failed to create image definition: %w", err)
		}
	}

	err = createImageVersion(ctx, clients, gallery, definitionName, imageVersion, resourceGroup, diskID, regions, azTags, verbose)
	if err != nil {
		return fmt.Errorf("failed to create image version: %w", err)
	}

	err = outputResults(ctx, clients, gallery, definitionName, imageVersion, resourceGroup, verbose)
	if err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	return nil
}

func archEnum(arch string) (armcompute.Architecture, error) {
	switch arch {
	case "x64", "amd64", "x86_64":
		return armcompute.ArchitectureX64, nil
	case "arm64", "aarch64":
		return armcompute.ArchitectureArm64, nil
	default:
		return *new(armcompute.Architecture), fmt.Errorf("unsupported architecture: %s", arch)
	}
}
