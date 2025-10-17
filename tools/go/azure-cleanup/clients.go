package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type AzureClients struct {
	Resources            *armresources.Client
	Disks                *armcompute.DisksClient
	Images               *armcompute.ImagesClient
	Galleries            *armcompute.GalleriesClient
	GalleryImages        *armcompute.GalleryImagesClient
	GalleryImageVersions *armcompute.GalleryImageVersionsClient
}

func createAzureClients(ctx context.Context, subscriptionID string) (*AzureClients, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	clients := &AzureClients{}

	clients.Resources, err = armresources.NewClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resources client: %w", err)
	}

	clients.Disks, err = armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	clients.Images, err = armcompute.NewImagesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create images client: %w", err)
	}

	clients.Galleries, err = armcompute.NewGalleriesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create galleries client: %w", err)
	}

	clients.GalleryImages, err = armcompute.NewGalleryImagesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gallery images client: %w", err)
	}

	clients.GalleryImageVersions, err = armcompute.NewGalleryImageVersionsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gallery image versions client: %w", err)
	}

	return clients, nil
}
