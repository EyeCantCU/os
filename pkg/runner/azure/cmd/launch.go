package cmd

import (
	"fmt"
	"log"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils"
	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/cobra"
)

func launchCmd() *cobra.Command {
	var subscriptionID string
	var region string
	var resourceGroup string
	var vmName string
	var vmSize string
	var publicKeyPath string
	var adminUsername string
	var imageURI string
	var vnetName string
	var subnetName string
	var tagName string

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch an Azure VM",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			resourceTags := map[string]*string{
				tagName: utils.ToPtr(""),
			}

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to obtain credential: %v", err)
			}

			pubKey, err := sshutils.GetSinglePublicKey(publicKeyPath)
			if err != nil {
				log.Fatalf("failed to read public key: %v", err)
			}

			// Create Public IP, derive name from VM name
			publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create Public IP client: %v", err)
			}

			publicIPName := fmt.Sprintf("ip-%s", vmName)
			log.Printf("Creating public IP: %s", publicIPName)
			_, err = publicIPClient.BeginCreateOrUpdate(ctx, resourceGroup, publicIPName, armnetwork.PublicIPAddress{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: utils.ToPtr(armnetwork.IPAllocationMethodDynamic),
				},
			}, nil)
			if err != nil {
				log.Fatalf("failed to create public IP: %v", err)
			}

			// Subnet and VNet
			subnetID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s", subscriptionID, resourceGroup, vnetName, subnetName)
			if subnetName == "" {
				subnetID = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/default", subscriptionID, resourceGroup, vnetName)
			}

			// Create NIC, derive name from VM name
			nicName := fmt.Sprintf("nic-%s", vmName)
			nicID := fmt.Sprintf(
				"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkInterfaces/%s",
				subscriptionID, resourceGroup, nicName,
			)
			nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create NIC client: %v", err)
			}

			_, err = nicClient.BeginCreateOrUpdate(ctx, resourceGroup, nicName, armnetwork.Interface{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armnetwork.InterfacePropertiesFormat{
					IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
						{
							Name: utils.ToPtr(fmt.Sprintf("ipconfig-%s", vmName)),
							Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
								PrivateIPAllocationMethod: utils.ToPtr(armnetwork.IPAllocationMethodDynamic),
								PublicIPAddress: &armnetwork.PublicIPAddress{
									ID: utils.ToPtr(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s", subscriptionID, resourceGroup, publicIPName)),
								},
								Subnet: &armnetwork.Subnet{
									ID: &subnetID,
								},
							},
						},
					},
				},
			}, nil)
			if err != nil {
				log.Fatalf("failed to create NIC: %v", err)
			}

			// Create VM
			vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create VM client: %v", err)
			}

			_, err = vmClient.BeginCreateOrUpdate(ctx, resourceGroup, vmName, armcompute.VirtualMachine{
				Location: &region,
				Tags:     resourceTags,
				Properties: &armcompute.VirtualMachineProperties{
					HardwareProfile: &armcompute.HardwareProfile{
						VMSize: utils.ToPtr(armcompute.VirtualMachineSizeTypes(vmSize)),
					},
					StorageProfile: &armcompute.StorageProfile{
						ImageReference: &armcompute.ImageReference{
							ID: &imageURI,
						},
					},
					OSProfile: &armcompute.OSProfile{
						ComputerName:  &vmName,
						AdminUsername: &adminUsername,
						LinuxConfiguration: &armcompute.LinuxConfiguration{
							DisablePasswordAuthentication: utils.ToPtr(true),
							SSH: &armcompute.SSHConfiguration{
								PublicKeys: []*armcompute.SSHPublicKey{
									{
										Path:    utils.ToPtr(sshutils.GetAuthorizedKeysPath(adminUsername)),
										KeyData: utils.ToPtr(string(pubKey)),
									},
								},
							},
						},
					},
					NetworkProfile: &armcompute.NetworkProfile{
						NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
							{
								ID: &nicID,
								Properties: &armcompute.NetworkInterfaceReferenceProperties{
									Primary: utils.ToPtr(true),
								},
							},
						},
					},
					DiagnosticsProfile: &armcompute.DiagnosticsProfile{
						BootDiagnostics: &armcompute.BootDiagnostics{
							Enabled: utils.ToPtr(true),
						},
					},
				},
			}, nil)
			if err != nil {
				log.Fatalf("failed to launch Azure VM: %v", err)
			}

			log.Printf("Launched Azure VM: %s", vmName)
		},
	}

	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVar(&region, "region", "", "Azure region")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")

	cmd.Flags().StringVar(&vmName, "name", "", "VM name (required)")
	cmd.Flags().StringVar(&vmSize, "vm-size", "Standard_B1s", "Azure VM size")
	cmd.Flags().StringVar(&adminUsername, "admin-username", "azureuser", "Admin username for SSH")
	cmd.Flags().StringVar(&publicKeyPath, "public-key", "id_rsa.pub", "Path to SSH public key")

	cmd.Flags().StringVar(&imageURI, "image-uri", "", "Image reference URI (required)")

	cmd.Flags().StringVar(&vnetName, "vnet", "", "VNet name (required)")
	cmd.Flags().StringVar(&subnetName, "subnet", "default", "Subnet name (optional)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag name to apply to resources (required)")

	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("location")
	cmd.MarkFlagRequired("subscription-id")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("image-uri")
	cmd.MarkFlagRequired("vnet")

	return cmd
}
