package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

type AzureClients struct {
	Disks            *armcompute.DisksClient
	Galleries        *armcompute.GalleriesClient
	ImageDefinitions *armcompute.GalleryImagesClient
	ImageVersions    *armcompute.GalleryImageVersionsClient
}

func createAzureClients(ctx context.Context, subscriptionID string) (*AzureClients, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	clients := &AzureClients{}

	clients.Disks, err = armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	clients.Galleries, err = armcompute.NewGalleriesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create galleries client: %w", err)
	}

	clients.ImageDefinitions, err = armcompute.NewGalleryImagesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image definitions client: %w", err)
	}

	clients.ImageVersions, err = armcompute.NewGalleryImageVersionsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image versions client: %w", err)
	}

	return clients, nil
}
