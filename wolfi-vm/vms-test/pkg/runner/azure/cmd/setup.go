package cmd

import (
	"log"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {
	var region, tagName, resourceGroupName, subscriptionID string
	var vnetName, subnetName, nsgName, routeTableName string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up Azure networking resources (VNet, Subnet, NSG, Route Table)",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			resourceTags := map[string]*string{
				tagName: utils.ToPtr(""),
			}

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to get Azure credentials: %v", err)
			}

			// Resource Group client
			rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create resource group client: %v", err)
			}

			// Check for existing resource group
			_, err = rgClient.Get(ctx, resourceGroupName, nil)
			if err != nil {
				log.Printf("Resource group %s not found. Creating...", resourceGroupName)
				_, err = rgClient.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{
					Tags:     resourceTags,
					Location: &region,
				}, nil)
				if err != nil {
					log.Fatalf("failed to create resource group: %v", err)
				}
			} else {
				log.Printf("Using existing resource group: %s", resourceGroupName)
			}

			// Create NSG
			nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create network security group client: %v", err)
			}
			nsgPoller, err := nsgClient.BeginCreateOrUpdate(ctx, resourceGroupName, nsgName, armnetwork.SecurityGroup{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armnetwork.SecurityGroupPropertiesFormat{
					SecurityRules: []*armnetwork.SecurityRule{
						{
							Name: utils.ToPtr("AllowSSH"),
							Properties: &armnetwork.SecurityRulePropertiesFormat{
								Access:                   utils.ToPtr(armnetwork.SecurityRuleAccessAllow),
								Direction:                utils.ToPtr(armnetwork.SecurityRuleDirectionInbound),
								Priority:                 utils.ToPtr(int32(100)),
								Protocol:                 utils.ToPtr(armnetwork.SecurityRuleProtocolTCP),
								SourceAddressPrefix:      utils.ToPtr("*"),
								DestinationAddressPrefix: utils.ToPtr("*"),
								SourcePortRange:          utils.ToPtr("*"),
								DestinationPortRange:     utils.ToPtr("22"),
							},
						},
					},
				},
			}, nil)
			nsgResp, _ := nsgPoller.PollUntilDone(ctx, nil)
			log.Printf("Created NSG: %s", *nsgResp.ID)

			// Create Route Table
			rtClient, err := armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create route tables client: %v", err)
			}
			rtPoller, err := rtClient.BeginCreateOrUpdate(ctx, resourceGroupName, routeTableName, armnetwork.RouteTable{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armnetwork.RouteTablePropertiesFormat{
					Routes: []*armnetwork.Route{
						{
							Name: utils.ToPtr("default-route"),
							Properties: &armnetwork.RoutePropertiesFormat{
								AddressPrefix: utils.ToPtr("0.0.0.0/0"),
								NextHopType:   utils.ToPtr(armnetwork.RouteNextHopTypeInternet),
							},
						},
					},
				},
			}, nil)
			rtResp, _ := rtPoller.PollUntilDone(ctx, nil)
			log.Printf("Created Route Table: %s", *rtResp.ID)

			// Create VNet
			vnetClient, _ := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
			vnetPoller, err := vnetClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, armnetwork.VirtualNetwork{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
					AddressSpace: &armnetwork.AddressSpace{
						AddressPrefixes: []*string{utils.ToPtr("10.0.0.0/24")},
					},
				},
			}, nil)
			vnetResp, _ := vnetPoller.PollUntilDone(ctx, nil)
			log.Printf("Created VNet: %s", *vnetResp.ID)

			// Create Subnet and associate NSG and Route Table
			subnetClient, _ := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
			subnetPoller, err := subnetClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, subnetName, armnetwork.Subnet{
				// Subnets don't support tags but they also get deleted automatically with the vNet so not a big deal.
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        utils.ToPtr("10.0.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: nsgResp.ID},
					RouteTable:           &armnetwork.RouteTable{ID: rtResp.ID},
				},
			}, nil)
			subnetResp, _ := subnetPoller.PollUntilDone(ctx, nil)
			log.Printf("Created Subnet with NSG and Route Table association: %s", *subnetResp.ID)

			log.Println("Azure network setup complete.")
		},
	}

	cmd.Flags().StringVar(&region, "region", "", "Azure region (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name to apply to resources (required)")
	cmd.Flags().StringVar(&resourceGroupName, "resource-group", "", "Resource group name (required)")
	cmd.Flags().StringVar(&vnetName, "vnet-name", "", "Virtual network name (required)")
	cmd.Flags().StringVar(&subnetName, "subnet-name", "", "Subnet name (required)")
	cmd.Flags().StringVar(&nsgName, "nsg-name", "", "Network security group name (required)")
	cmd.Flags().StringVar(&routeTableName, "route-table-name", "", "Route table name (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")

	cmd.MarkFlagRequired("region")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("vnet-name")
	cmd.MarkFlagRequired("subnet-name")
	cmd.MarkFlagRequired("nsg-name")
	cmd.MarkFlagRequired("route-table-name")
	cmd.MarkFlagRequired("subscription-id")

	return cmd
}
