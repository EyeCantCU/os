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

func createImageDefinition(ctx context.Context, clients *AzureClients, galleryName, definitionName, resourceGroup, region string, arch armcompute.Architecture, tags map[string]*string, verbose bool) error {
	imageDefinition := armcompute.GalleryImage{
		Location: to.Ptr(region),
		Properties: &armcompute.GalleryImageProperties{
			OSType:       to.Ptr(armcompute.OperatingSystemTypesLinux),
			OSState:      to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
			Architecture: to.Ptr(arch),
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: to.Ptr(defaultPublisher),
				Offer:     to.Ptr(defaultOffer),
				SKU:       to.Ptr(definitionName),
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
		Tags: tags,
	}

	if verbose {
		log.Printf("Creating image definition: %s in gallery: %s", definitionName, galleryName)
	}

	poller, err := clients.ImageDefinitions.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, definitionName, imageDefinition, nil)
	if err != nil {
		return fmt.Errorf("failed to start image definition creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create image definition: %w", err)
	}

	if verbose {
		log.Printf("Image definition created successfully")
	}

	return nil
}

func createImageVersion(ctx context.Context, clients *AzureClients, galleryName, definitionName, version, resourceGroup, diskID string, regions []string, tags map[string]*string, verbose bool) error {
	targetRegions := make([]*armcompute.TargetRegion, 0, len(regions))
	for _, region := range regions {
		targetRegions = append(targetRegions, &armcompute.TargetRegion{
			Name: to.Ptr(region),
		})
	}

	imageVersion := armcompute.GalleryImageVersion{
		Location: to.Ptr(regions[0]),
		Properties: &armcompute.GalleryImageVersionProperties{
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				OSDiskImage: &armcompute.GalleryOSDiskImage{
					Source: &armcompute.GalleryDiskImageSource{
						ID: to.Ptr(diskID),
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
		Tags: tags,
	}

	if verbose {
		log.Printf("Creating image version: %s for definition: %s", version, definitionName)
	}

	poller, err := clients.ImageVersions.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, definitionName, version, imageVersion, nil)
	if err != nil {
		return fmt.Errorf("failed to start image version creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create image version: %w", err)
	}

	if verbose {
		log.Printf("Image version created successfully")
	}

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
