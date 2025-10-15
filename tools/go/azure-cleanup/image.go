package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func getImages(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.Images.NewListByResourceGroupPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, image := range page.Value {
			if image == nil || image.Name == nil || image.ID == nil {
				continue
			}

			// For gallery images, find creation time of newest image version
			// Assumption: the latest version has the newest creation time
			var createdTime *time.Time
			if strings.Contains(*image.Type, "galleries/images") {
				// This is a gallery image, get the newest version's creation time
				// Parse gallery and image name from ID
				idParts := strings.Split(*image.ID, "/")
				if len(idParts) >= 10 {
					galleryName := idParts[8] // .../galleries/{galleryName}/...
					imageName := idParts[10]  // .../images/{imageName}

					// Get latest version directly
					latestVersionResp, err := clients.GalleryImageVersions.Get(ctx, resourceGroup, galleryName, imageName, "latest", nil)
					var latestVersion *armcompute.GalleryImageVersion
					if err == nil {
						latestVersion = &latestVersionResp.GalleryImageVersion
					}

					if latestVersion != nil && latestVersion.Properties != nil && latestVersion.Properties.PublishingProfile != nil {
						createdTime = latestVersion.Properties.PublishingProfile.PublishedDate
					}
				}
			}

			if createdTime == nil {
				createdTime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *image.ID,
				Name:         *image.Name,
				Type:         "Microsoft.Compute/images",
				CreatedTime:  createdTime,
				Tags:         image.Tags,
				ResourceType: ResourceTypeImage,
			})
		}
	}

	return resources, nil
}

func deleteImage(ctx context.Context, client *armcompute.ImagesClient, resourceName string) error {
	poller, err := client.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func getImageVersions(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo

	// First get all galleries in the resource group
	galleryPager := clients.Galleries.NewListByResourceGroupPager(resourceGroup, nil)

	for galleryPager.More() {
		galleryPage, err := galleryPager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list galleries: %w", err)
		}

		for _, gallery := range galleryPage.Value {
			if gallery == nil || gallery.Name == nil {
				continue
			}

			// Get all images in this gallery
			imagePager := clients.GalleryImages.NewListByGalleryPager(resourceGroup, *gallery.Name, nil)

			for imagePager.More() {
				imagePage, err := imagePager.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to list gallery images: %w", err)
				}

				for _, image := range imagePage.Value {
					if image == nil || image.Name == nil {
						continue
					}

					// Get all versions of this image
					versionPager := clients.GalleryImageVersions.NewListByGalleryImagePager(resourceGroup, *gallery.Name, *image.Name, nil)

					for versionPager.More() {
						versionPage, err := versionPager.NextPage(ctx)
						if err != nil {
							return nil, fmt.Errorf("failed to list gallery image versions: %w", err)
						}

						for _, version := range versionPage.Value {
							if version == nil || version.Name == nil || version.ID == nil {
								continue
							}

							var createdTime *time.Time
							if version.Properties != nil && version.Properties.PublishingProfile != nil && version.Properties.PublishingProfile.PublishedDate != nil {
								createdTime = version.Properties.PublishingProfile.PublishedDate
							} else {
								createdTime = unixZero()
							}

							resources = append(resources, &ResourceInfo{
								ID:           *version.ID,
								Name:         fmt.Sprintf("%s/%s/%s", *gallery.Name, *image.Name, *version.Name),
								Type:         "Microsoft.Compute/galleries/images/versions",
								CreatedTime:  createdTime,
								Tags:         version.Tags,
								ResourceType: ResourceTypeImageVersion,
							})
						}
					}
				}
			}
		}
	}

	return resources, nil
}

func deleteImageVersion(ctx context.Context, clients *AzureClients, resource *ResourceInfo) error {
	// Parse the resource name: galleryName/imageName/versionName
	parts := strings.Split(resource.Name, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid gallery image version name format: %s", resource.Name)
	}

	galleryName := parts[0]
	imageName := parts[1]
	versionName := parts[2]

	poller, err := clients.GalleryImageVersions.BeginDelete(ctx, resourceGroup, galleryName, imageName, versionName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
