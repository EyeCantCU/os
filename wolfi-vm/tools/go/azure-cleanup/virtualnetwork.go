package main

import (
	"context"
	"strings"
	"time"
)

func getVirtualNetworks(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.VNet.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, vnet := range page.Value {
			if vnet == nil || vnet.Name == nil || vnet.ID == nil {
				continue
			}

			// Map VNet creation time to VMs in subnets
			var createdTime *time.Time
			if vnet.Properties != nil && vnet.Properties.Subnets != nil {
				for _, subnet := range vnet.Properties.Subnets {
					if subnet == nil || subnet.Properties == nil || subnet.Properties.IPConfigurations == nil {
						continue
					}

					for _, ipConfig := range subnet.Properties.IPConfigurations {
						if ipConfig == nil || ipConfig.ID == nil {
							continue
						}

						// Parse IP configuration to get network interface
						ipConfigParts := strings.Split(*ipConfig.ID, "/")
						if len(ipConfigParts) >= 9 {
							niResourceGroup := ipConfigParts[4]
							niName := ipConfigParts[8]

							// Get network interface to find VM
							ni, err := clients.NetworkInterfaces.Get(ctx, niResourceGroup, niName, nil)
							if err != nil {
								continue
							}

							if ni.Properties != nil && ni.Properties.VirtualMachine != nil && ni.Properties.VirtualMachine.ID != nil {
								vmTime := getVMCreationTime(ctx, clients, *ni.Properties.VirtualMachine.ID)
								if vmTime != nil {
									if createdTime == nil || vmTime.After(*createdTime) {
										createdTime = vmTime // Use newest VM time
									}
								}
							}
						}
					}
				}
			}

			if createdTime == nil {
				createdTime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *vnet.ID,
				Name:         *vnet.Name,
				Type:         "Microsoft.Network/virtualNetworks",
				CreatedTime:  createdTime,
				Tags:         vnet.Tags,
				ResourceType: ResourceTypeVnet,
			})
		}
	}

	return resources, nil
}

func deleteVirtualNetwork(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.VNet.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
