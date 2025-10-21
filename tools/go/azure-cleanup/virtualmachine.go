package main

import (
	"context"
	"log"
	"strings"
	"time"
)

func getVirtualMachines(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.VirtualMachines.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, vm := range page.Value {
			if vm == nil || vm.Name == nil || vm.ID == nil {
				continue
			}

			resources = append(resources, &ResourceInfo{
				ID:           *vm.ID,
				Name:         *vm.Name,
				Type:         "Microsoft.Compute/virtualMachines",
				CreatedTime:  vm.Properties.TimeCreated,
				Tags:         vm.Tags,
				ResourceType: ResourceTypeVM,
			})
		}
	}

	return resources, nil
}

func deleteVirtualMachine(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.VirtualMachines.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func getVMCreationTime(ctx context.Context, clients *AzureClients, vmID string) *time.Time {
	// /subscriptions/{subscription-id}/resourceGroups/{resource-group}/providers/Microsoft.Compute/virtualMachines/{vm-name}
	parts := strings.Split(vmID, "/")
	if len(parts) < 9 {
		log.Printf("Invalid VM ID format: %s", vmID)
		return nil
	}

	vmResourceGroup := parts[4]
	vmName := parts[8]

	// Return nil if we know this is attached to a VM but fail to look it up,
	// so that we don't try to delete a NIC that's in use.
	vm, err := clients.VirtualMachines.Get(ctx, vmResourceGroup, vmName, nil)
	if err != nil {
		log.Printf("Failed to get VM %s in resource group %s: %v", vmName, vmResourceGroup, err)
		return nil
	}

	if vm.Properties != nil {
		return vm.Properties.TimeCreated
	}

	return nil
}
