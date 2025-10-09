package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

type createImageDefinitionOpts struct {
	galleryName           string
	definitionName        string
	resourceGroup         string
	region                string
	arch                  armcompute.Architecture
	tags                  map[string]*string
	acceleratedNetworking bool
}

func createImageDefinition(ctx context.Context, clients *AzureClients, opts *createImageDefinitionOpts, verbose bool) error {
	imageDefinition := armcompute.GalleryImage{
		Location: to.Ptr(opts.region),
		Properties: &armcompute.GalleryImageProperties{
			OSType:       to.Ptr(armcompute.OperatingSystemTypesLinux),
			OSState:      to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
			Architecture: to.Ptr(opts.arch),
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: to.Ptr(defaultPublisher),
				Offer:     to.Ptr(defaultOffer),
				SKU:       to.Ptr(opts.definitionName),
			},
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationV2),
			Features:         []*armcompute.GalleryImageFeature{
				// Setting SecurityType = Standard returns an error from the Azure backend that this is not a valid value.
				// In the latest version (v7) of the API, the SecurityTypes enum does not contain a 'Standard' value, only
				// values for trusted (secure boot) and confidential compute.
				//
				// I dislike not specifying this but I uploaded a new definition and it does default to standard, and is
				// also capable of updating old definitions which previously specified standard explicitly with az cli.
				/*{
					Name: to.Ptr("SecurityType"),
					Value: to.Ptr("TrustedLaunchSupported"),
				},*/
			},
		},
		Tags: opts.tags,
	}

	if opts.acceleratedNetworking {
		feature := &armcompute.GalleryImageFeature{
			Name:  to.Ptr("IsAcceleratedNetworkSupported"),
			Value: to.Ptr("true"),
		}
		imageDefinition.Properties.Features = append(imageDefinition.Properties.Features, feature)
	}

	log.Printf("Creating image definition: %s in gallery: %s", definitionName, opts.galleryName)

	poller, err := clients.ImageDefinitions.BeginCreateOrUpdate(ctx, resourceGroup, opts.galleryName, opts.definitionName, imageDefinition, nil)
	if err != nil {
		return fmt.Errorf("failed to start image definition creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create image definition: %w", err)
	}

	log.Printf("Image definition created successfully")

	return nil
}

type createImageVersionOpts struct {
	galleryName    string
	definitionName string
	version        string
	resourceGroup  string
	diskID         string
	regions        []string
	tags           map[string]*string
}

func createImageVersion(ctx context.Context, clients *AzureClients, opts *createImageVersionOpts, verbose bool) error {
	targetRegions := make([]*armcompute.TargetRegion, 0, len(opts.regions))
	for _, region := range opts.regions {
		targetRegions = append(targetRegions, &armcompute.TargetRegion{
			Name: to.Ptr(region),
		})
	}

	imageVersion := armcompute.GalleryImageVersion{
		Location: to.Ptr(opts.regions[0]),
		Properties: &armcompute.GalleryImageVersionProperties{
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				OSDiskImage: &armcompute.GalleryOSDiskImage{
					Source: &armcompute.GalleryDiskImageSource{
						ID: to.Ptr(opts.diskID),
					},
				},
			},
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				TargetRegions: targetRegions,
			},
			SecurityProfile: &armcompute.ImageVersionSecurityProfile{
				/*UefiSettings: &armcompute.GalleryImageVersionUefiSettings{
					            SignatureTemplateNames: []*armcompute.UefiSignatureTemplateName{
						            to.Ptr(armcompute.UefiSignatureTemplateNameNoSignatureTemplate),
					            },
				            },*/
			},
		},
		Tags: opts.tags,
	}

	log.Printf("Creating image version: %s for definition: %s", opts.version, opts.definitionName)

	poller, err := clients.ImageVersions.BeginCreateOrUpdate(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, opts.version, imageVersion, nil)
	if err != nil {
		return fmt.Errorf("failed to start image version creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create image version: %w", err)
	}

	log.Printf("Image version created successfully")

	return nil
}

func outputResults(ctx context.Context, clients *AzureClients, galleryName, definitionName, version, resourceGroup string, verbose bool) error {
	imageDefinition, err := clients.ImageDefinitions.Get(ctx, resourceGroup, galleryName, definitionName, nil)
	if err != nil {
		return fmt.Errorf("failed to get image definition: %w", err)
	}

	imageVersion, err := clients.ImageVersions.Get(ctx, resourceGroup, galleryName, definitionName, version, nil)
	if err != nil {
		return fmt.Errorf("failed to get image version: %w", err)
	}

	encoder := json.NewEncoder(os.Stdout)

	if err := encoder.Encode(imageDefinition); err != nil {
		return fmt.Errorf("failed to encode image definition: %w", err)
	}

	if err := encoder.Encode(imageVersion); err != nil {
		return fmt.Errorf("failed to encode image version: %w", err)
	}

	return nil
}
