package main

import (
	"context"
)

func getDisks(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.Disks.NewListByResourceGroupPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, disk := range page.Value {
			if disk == nil || disk.Name == nil || disk.ID == nil {
				continue
			}

			resources = append(resources, &ResourceInfo{
				ID:           *disk.ID,
				Name:         *disk.Name,
				Type:         "Microsoft.Compute/disks",
				CreatedTime:  disk.Properties.TimeCreated,
				Tags:         disk.Tags,
				ResourceType: ResourceTypeDisk,
			})
		}
	}

	return resources, nil
}

func deleteDisk(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.Disks.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
