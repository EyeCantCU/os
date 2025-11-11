package main

import (
	"context"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

func getPublicIPAddresses(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.PublicIP.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, pip := range page.Value {
			if pip == nil || pip.Name == nil || pip.ID == nil {
				continue
			}

			// Check delete option and map creation time to attached VM
			var createdTime *time.Time
			if pip.Properties != nil {
				// If DeleteOption is Delete, this will be deleted with the VM so filter it out.
				if pip.Properties.DeleteOption != nil && *pip.Properties.DeleteOption == armnetwork.DeleteOptionsDelete {
					createdTime = nil
				} else {
					// Map to VM via network interface configuration
					if pip.Properties.IPConfiguration != nil && pip.Properties.IPConfiguration.ID != nil {
						// Parse network interface ID from IP configuration
						configParts := strings.Split(*pip.Properties.IPConfiguration.ID, "/")
						if len(configParts) >= 9 {
							niResourceGroup := configParts[4]
							niName := configParts[8]

							// Get network interface to find VM
							ni, err := clients.NetworkInterfaces.Get(ctx, niResourceGroup, niName, nil)
							if err == nil && ni.Properties != nil && ni.Properties.VirtualMachine != nil && ni.Properties.VirtualMachine.ID != nil {
								createdTime = getVMCreationTime(ctx, clients, *ni.Properties.VirtualMachine.ID)
							}
						}
					}
				}
			}

			if createdTime == nil {
				createdTime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *pip.ID,
				Name:         *pip.Name,
				Type:         "Microsoft.Network/publicIPAddresses",
				CreatedTime:  createdTime,
				Tags:         pip.Tags,
				ResourceType: ResourceTypeIP,
			})
		}
	}

	return resources, nil
}

func deletePublicIPAddress(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.PublicIP.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
