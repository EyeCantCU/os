package cmd

import (
	"log"
	"strings"

	azruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/cobra"
)

func teardownCmd() *cobra.Command {
	var subscriptionID, resourceGroup, tagName string
	var userRequestedRGDeletion bool

	cmd := &cobra.Command{
		Use:   "teardown",
		Short: "Delete Azure vNet, NSGs, and route tables by tag.",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to create credential: %v", err)
			}

			rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create resource group client: %v", err)
			}

			rgResp, err := rgClient.Get(ctx, resourceGroup, nil)
			if err != nil {
				log.Fatalf("failed to get resource group: %v", err)
			}

			deleteResourceGroup := userRequestedRGDeletion
			if !deleteResourceGroup && rgResp.Tags != nil {
				if _, ok := rgResp.Tags[tagName]; ok {
					deleteResourceGroup = true
				}
			}
			if deleteResourceGroup {
				log.Printf("Deleting resource group: %s", resourceGroup)
				poller, err := rgClient.BeginDelete(ctx, resourceGroup, nil)
				if err != nil {
					log.Fatalf("failed to delete resource group: %v", err)
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for resource group %s deletion: %v", resourceGroup, err)
				} else {
					log.Printf("Deleted resource group: %s", resourceGroup)
				}
				return
			}

			resClient, err := armresources.NewClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create resource client: %v", err)
			}

			pager := resClient.NewListByResourceGroupPager(resourceGroup, nil)

			var vnets, nsgs, routeTables []string

			for pager.More() {
				resp, err := pager.NextPage(ctx)
				if err != nil {
					log.Fatalf("failed to list resources: %v", err)
				}

				for _, res := range resp.Value {
					if res.Tags == nil || res.ID == nil {
						continue
					}
					if _, ok := res.Tags[tagName]; !ok {
						continue
					}

					idParts := strings.Split(*res.ID, "/")
					if len(idParts) < 9 {
						continue
					}
					resourceType := idParts[7]
					resourceName := idParts[8]

					switch resourceType {
					case "virtualNetworks":
						vnets = append(vnets, resourceName)
					case "networkSecurityGroups":
						nsgs = append(nsgs, resourceName)
					case "routeTables":
						routeTables = append(routeTables, resourceName)
					}
				}
			}

			// --- Delete vNets ---
			vnetClient, _ := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
			var vnetPollers []*azruntime.Poller[armnetwork.VirtualNetworksClientDeleteResponse]
			for _, name := range vnets {
				log.Printf("Deleting vNet: %s", name)
				poller, err := vnetClient.BeginDelete(ctx, resourceGroup, name, nil)
				if err != nil {
					log.Printf("failed to delete vNet %s: %v", name, err)
					continue
				}
				vnetPollers = append(vnetPollers, poller)
			}
			for i, poller := range vnetPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for vNet %s deletion: %v", vnets[i], err)
				} else {
					log.Printf("Deleted vNet: %s", vnets[i])
				}
			}

			// --- Delete NSGs ---
			nsgClient, _ := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
			var nsgPollers []*azruntime.Poller[armnetwork.SecurityGroupsClientDeleteResponse]
			for _, name := range nsgs {
				log.Printf("Deleting NSG: %s", name)
				poller, err := nsgClient.BeginDelete(ctx, resourceGroup, name, nil)
				if err != nil {
					log.Printf("failed to delete NSG %s: %v", name, err)
					continue
				}
				nsgPollers = append(nsgPollers, poller)
			}
			for i, poller := range nsgPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for NSG %s deletion: %v", nsgs[i], err)
				} else {
					log.Printf("Deleted NSG: %s", nsgs[i])
				}
			}

			// --- Delete Route Tables ---
			rtClient, _ := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
			var rtPollers []*azruntime.Poller[armnetwork.RouteTablesClientDeleteResponse]
			for _, name := range routeTables {
				log.Printf("Deleting route table: %s", name)
				poller, err := rtClient.BeginDelete(ctx, resourceGroup, name, nil)
				if err != nil {
					log.Printf("failed to delete route table %s: %v", name, err)
					continue
				}
				rtPollers = append(rtPollers, poller)
			}
			for i, poller := range rtPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for route table %s deletion: %v", routeTables[i], err)
				} else {
					log.Printf("Deleted route table: %s", routeTables[i])
				}
			}
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag key to identify resources (required)")
	cmd.Flags().BoolVar(&userRequestedRGDeletion, "always-delete-resource-group", false, "Delete the whole resource group (if false: deleted when tag matches)")
	cmd.MarkFlagRequired("subscription-id")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")

	return cmd
}
