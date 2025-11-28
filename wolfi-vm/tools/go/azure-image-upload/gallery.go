package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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
	deleteIfNecessary     bool
	deleteJobs            int
	trustedLaunch         bool
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

	if opts.trustedLaunch {
		feature := &armcompute.GalleryImageFeature{
			Name:  to.Ptr("SecurityType"),
			Value: to.Ptr("TrustedLaunchSupported"),
		}
		imageDefinition.Properties.Features = append(imageDefinition.Properties.Features, feature)
	}

	retryAfterDelete := func() error {
		log.Printf("Image definition conflict detected. Deleting existing definition and recreating...")

		// First list image versions
		log.Printf("Listing image versions for definition: %s", opts.definitionName)
		pager := clients.ImageVersions.NewListByGalleryImagePager(opts.resourceGroup, opts.galleryName, opts.definitionName, nil)

		var versionNames []string
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list image versions: %w", err)
			}

			for _, version := range page.Value {
				if version.Name != nil {
					versionNames = append(versionNames, *version.Name)
				}
			}
		}

		// Delete all image versions in parallel
		if len(versionNames) > 0 {
			log.Printf("Deleting %d image versions in parallel", len(versionNames))
			errChan := make(chan error, len(versionNames))

			// Stick job IDs in a queue
			jobChan := make(chan int, opts.deleteJobs)
			for i := 0; i < opts.deleteJobs; i++ {
				jobChan <- i
			}

			for _, versionName := range versionNames {
				go func(vName string) {
					// Wait for available job ID
					id := <-jobChan
					// Put job ID back in queue for other image versions
					defer func() { jobChan <- id }()
					log.Printf("Deleting image version: %s in job %d", vName, id)

					versionDeletePoller, err := clients.ImageVersions.BeginDelete(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, vName, nil)
					if err != nil {
						log.Printf("Failed to start deletion of image version %s: %v", vName, err)
						errChan <- nil
						return
					}

					_, err = versionDeletePoller.PollUntilDone(ctx, nil)
					if err != nil {
						log.Printf("Failed to delete image version %s: %v", vName, err)
					} else {
						log.Printf("Image version %s deleted successfully", vName)
					}
					errChan <- nil
				}(versionName)
			}

			for i := 0; i < len(versionNames); i++ {
				// Do not exit early on error. Once we've started
				// destroying data, attempt to continue destroying everything
				// so that we can create the new image definitions.
				<-errChan
			}
		}

		// Now delete the image definition
		log.Printf("Deleting image definition: %s", opts.definitionName)
		deletePoller, err := clients.ImageDefinitions.BeginDelete(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, nil)
		if err != nil {
			log.Printf("Failed to start image definition deletion: %v", err)
		} else {
			_, err = deletePoller.PollUntilDone(ctx, nil)
			if err != nil {
				log.Printf("Failed to delete image definition: %v", err)
			} else {
				log.Printf("Image definition deleted successfully")
			}
		}

		log.Printf("Creating new image definition...")
		poller, err := clients.ImageDefinitions.BeginCreateOrUpdate(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, imageDefinition, nil)
		if err != nil {
			return fmt.Errorf("failed to start image definition creation after deletion: %w\nTHIS IS VERY BAD", err)
		}
		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to create image definition after deletion: %w\nTHIS IS VERY BAD", err)
		}
		return nil
	}

	log.Printf("Creating image definition: %s in gallery: %s", opts.definitionName, opts.galleryName)

	poller, err := clients.ImageDefinitions.BeginCreateOrUpdate(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, imageDefinition, nil)
	if err != nil {
		if opts.deleteIfNecessary && isConflictError(err) {
			return retryAfterDelete()
		} else {
			return fmt.Errorf("failed to start image definition creation: %w", err)
		}
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		if opts.deleteIfNecessary && isConflictError(err) {
			return retryAfterDelete()
		} else {
			return fmt.Errorf("failed to create image definition after deletion: %w\nTHIS IS VERY BAD", err)
		}
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
	dbHashes       []string
	Attempts       int
	AttemptBackoff time.Duration
}

func createImageVersion(ctx context.Context, clients *AzureClients, opts *createImageVersionOpts, verbose bool) error {
	targetRegions := make([]*armcompute.TargetRegion, 0, len(opts.regions))
	for _, region := range opts.regions {
		targetRegions = append(targetRegions, &armcompute.TargetRegion{
			Name: to.Ptr(region),
		})
	}
	targetDb := make([]*string, 0, len(opts.dbHashes))
	for _, dbHash := range opts.dbHashes {
		targetDb = append(targetDb, to.Ptr(dbHash))
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
		},
		Tags: opts.tags,
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

	log.Printf("Creating image version: %s for definition: %s", opts.version, opts.definitionName)

	makeVer := func() (*runtime.Poller[armcompute.GalleryImageVersionsClientCreateOrUpdateResponse], error) {
		return clients.ImageVersions.BeginCreateOrUpdate(ctx, opts.resourceGroup, opts.galleryName, opts.definitionName, opts.version, imageVersion, nil)
	}

	poller, err := tryAPICall(makeVer, opts.Attempts, opts.AttemptBackoff)
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

// isConflictError checks if the error indicates a 409 conflict error
func isConflictError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for HTTP 409 status code in the error message
	return strings.Contains(errStr, "409") || strings.Contains(strings.ToLower(errStr), "conflict")
}
