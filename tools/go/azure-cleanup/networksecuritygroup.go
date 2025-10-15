package main

import (
	"context"
	"time"
)

func getNetworkSecurityGroups(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.NetworkSecurityGroup.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, nsg := range page.Value {
			if nsg == nil || nsg.Name == nil || nsg.ID == nil {
				continue
			}

			// Map NSG creation time to VMs via network interfaces
			var createdTime *time.Time
			if nsg.Properties != nil && nsg.Properties.NetworkInterfaces != nil {
				for _, niRef := range nsg.Properties.NetworkInterfaces {
					if niRef == nil || niRef.Properties == nil {
						continue
					}

					if niRef.Properties.VirtualMachine != nil && niRef.Properties.VirtualMachine.ID != nil {
						vmTime := getVMCreationTime(ctx, clients, *niRef.Properties.VirtualMachine.ID)
						if vmTime != nil {
							if createdTime == nil || vmTime.After(*createdTime) {
								createdTime = vmTime // Use newest VM time
							}
						}
					}
				}
			}

			if createdTime == nil {
				createdTime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *nsg.ID,
				Name:         *nsg.Name,
				Type:         "Microsoft.Network/networkSecurityGroups",
				CreatedTime:  createdTime,
				Tags:         nsg.Tags,
				ResourceType: ResourceTypeNetworkSecurityGroup,
			})
		}
	}

	return resources, nil
}

func deleteNetworkSecurityGroup(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.NetworkSecurityGroup.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
