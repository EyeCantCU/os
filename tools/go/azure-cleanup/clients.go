package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type AzureClients struct {
	Resources            *armresources.Client
	Disks                *armcompute.DisksClient
	Images               *armcompute.ImagesClient
	Galleries            *armcompute.GalleriesClient
	GalleryImages        *armcompute.GalleryImagesClient
	GalleryImageVersions *armcompute.GalleryImageVersionsClient
	VirtualMachines      *armcompute.VirtualMachinesClient
	PublicIP             *armnetwork.PublicIPAddressesClient
	NetworkInterfaces    *armnetwork.InterfacesClient
	VNet                 *armnetwork.VirtualNetworksClient
	RouteTable           *armnetwork.RouteTablesClient
	Subnet               *armnetwork.SubnetsClient
	NetworkSecurityGroup *armnetwork.SecurityGroupsClient
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

	clients.VirtualMachines, err = armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual machines client: %w", err)
	}

	clients.PublicIP, err = armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP addresses client: %w", err)
	}

	clients.NetworkInterfaces, err = armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interfaces client: %w", err)
	}

	clients.VNet, err = armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual networks client: %w", err)
	}

	clients.RouteTable, err = armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create route tables client: %w", err)
	}

	clients.Subnet, err = armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnets client: %w", err)
	}

	clients.NetworkSecurityGroup, err = armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security groups client: %w", err)
	}

	return clients, nil
}
