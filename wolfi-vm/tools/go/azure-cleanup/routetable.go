package main

import (
	"context"
	"strings"
	"time"
)

func getRouteTables(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var resources []*ResourceInfo
	pager := clients.RouteTable.NewListPager(resourceGroup, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, routeTable := range page.Value {
			if routeTable == nil || routeTable.Name == nil || routeTable.ID == nil {
				continue
			}

			// Map Route Table creation time to VMs via associated subnets
			var createdTime *time.Time
			if routeTable.Properties != nil && routeTable.Properties.Subnets != nil {
				for _, subnetRef := range routeTable.Properties.Subnets {
					if subnetRef == nil || subnetRef.ID == nil {
						continue
					}

					// Parse subnet ID to get VNet and subnet names
					subnetParts := strings.Split(*subnetRef.ID, "/")
					if len(subnetParts) >= 11 {
						vnetResourceGroup := subnetParts[4]
						vnetName := subnetParts[8]
						subnetName := subnetParts[10]

						// Get subnet to find network interfaces
						subnet, err := clients.Subnet.Get(ctx, vnetResourceGroup, vnetName, subnetName, nil)
						if err != nil {
							continue
						}

						if subnet.Properties != nil && subnet.Properties.IPConfigurations != nil {
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
				}
			}

			if createdTime == nil {
				createdTime = unixZero()
			}

			resources = append(resources, &ResourceInfo{
				ID:           *routeTable.ID,
				Name:         *routeTable.Name,
				Type:         "Microsoft.Network/routeTables",
				CreatedTime:  createdTime,
				Tags:         routeTable.Tags,
				ResourceType: ResourceTypeRouteTable,
			})
		}
	}

	return resources, nil
}

func deleteRouteTable(ctx context.Context, clients *AzureClients, resourceName string) error {
	poller, err := clients.RouteTable.BeginDelete(ctx, resourceGroup, resourceName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
