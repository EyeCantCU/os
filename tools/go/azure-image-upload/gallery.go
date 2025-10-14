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

func createImageDefinition(ctx context.Context, clients *AzureClients, galleryName, definitionName, resourceGroup, region string, arch armcompute.Architecture, tags map[string]*string, trustedLaunch, verbose bool) error {
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
		},
		Tags: tags,
	}

	if trustedLaunch {
		imageDefinition.Properties.Features = []*armcompute.GalleryImageFeature{
			{
				Name:  to.Ptr("SecurityType"),
				Value: to.Ptr("TrustedLaunchSupported"),
			},
		}
	}

	log.Printf("Creating image definition: %s in gallery: %s", definitionName, galleryName)

	poller, err := clients.ImageDefinitions.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, definitionName, imageDefinition, nil)
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

func createImageVersion(ctx context.Context, clients *AzureClients, galleryName, definitionName, version, resourceGroup, diskID string, regions []string, tags map[string]*string, dbHashes []string, verbose bool) error {
	targetRegions := make([]*armcompute.TargetRegion, 0, len(regions))
	for _, region := range regions {
		targetRegions = append(targetRegions, &armcompute.TargetRegion{
			Name: to.Ptr(region),
		})
	}
	targetDb := make([]*string, 0, len(dbHashes))
	for _, dbHash := range dbHashes {
		targetDb = append(targetDb, to.Ptr(dbHash))
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
		},
		Tags: tags,
	}

	if len(targetDb) > 0 {
		imageVersion.Properties.SecurityProfile = &armcompute.ImageVersionSecurityProfile{
			UefiSettings: &armcompute.GalleryImageVersionUefiSettings{
				AdditionalSignatures: &armcompute.UefiKeySignatures{
					Db: []*armcompute.UefiKey{
						{
							Type:  to.Ptr(armcompute.UefiKeyTypeSHA256),
							Value: targetDb,
						},
					},
				},
				SignatureTemplateNames: []*armcompute.UefiSignatureTemplateName{
					to.Ptr(armcompute.UefiSignatureTemplateNameMicrosoftUefiCertificateAuthorityTemplate),
				},
			},
		}
	}

	log.Printf("Creating image version: %s for definition: %s", version, definitionName)

	poller, err := clients.ImageVersions.BeginCreateOrUpdate(ctx, resourceGroup, galleryName, definitionName, version, imageVersion, nil)
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
