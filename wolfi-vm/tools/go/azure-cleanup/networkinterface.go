package main

import (
	"context"
	"time"
)

func getNetworkInterfaces(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.NetworkInterfaces.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, nic := range page.Value {
			if nic == nil || nic.Name == nil || nic.ID == nil {
				continue
			}

			var ctime *time.Time
			if nic.Properties != nil && nic.Properties.VirtualMachine != nil && nic.Properties.VirtualMachine.ID != nil {
				ctime = getVMCreationTime(ctx, clients, *nic.Properties.VirtualMachine.ID)
			} else {
				// If it's not attached to any VM, assume it's really old so it doesn't
				// stick around forever.
				ctime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *nic.ID,
				Name:         *nic.Name,
				Type:         "Microsoft.Network/networkInterfaces",
				CreatedTime:  ctime,
				Tags:         nic.Tags,
				ResourceType: ResourceTypeNIC,
			})
		}
	}

	return resources, nil
}

func deleteNetworkInterface(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.NetworkInterfaces.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
