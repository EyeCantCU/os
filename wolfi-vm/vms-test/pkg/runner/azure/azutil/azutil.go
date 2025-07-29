package azutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

// Return first instance with a tag
func GetFirstInstanceByTag(ctx context.Context, cred *azidentity.DefaultAzureCredential, subscriptionID, resourceGroup, vmTag string) (*armcompute.VirtualMachine, error) {
	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM client: %v", err)
	}

	pager := vmClient.NewListPager(resourceGroup, nil)

	var taggedVM *armcompute.VirtualMachine

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get VM list: %v", err)
		}
		for _, vm := range page.Value {
			if vm.Tags != nil {
				if _, ok := vm.Tags[vmTag]; ok {
					taggedVM = vm
					break
				}
			}
		}
	}
	if taggedVM == nil || *taggedVM.Name == "" {
		return nil, fmt.Errorf("never found a VM with tag %s", vmTag)
	}
	return taggedVM, nil
}

// Return primary public IP address of the first VM with a tag
func GetIPByTag(ctx context.Context, cred *azidentity.DefaultAzureCredential, subscriptionID, resourceGroup, vmTag string) (string, error) {
	vm, err := GetFirstInstanceByTag(ctx, cred, subscriptionID, resourceGroup, vmTag)
	if err != nil {
		return "", err
	}
	if len(vm.Properties.NetworkProfile.NetworkInterfaces) < 1 {
		return "", fmt.Errorf("vm %s with tag %s has no NICs", *vm.Name, vmTag)
	}
	nicID := *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID

	parts := strings.Split(nicID, "/")
	if len(parts) < 9 {
		return "", fmt.Errorf("unexpected NIC ID format: %s", nicID)
	}
	nicRG := parts[4]
	nicName := parts[8]

	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create NIC client: %v", err)
	}

	nic, err := nicClient.Get(ctx, nicRG, nicName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get NIC: %v", err)
	}

	if nic.Properties == nil || nic.Properties.IPConfigurations == nil || len(nic.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("NIC has no IP configuration")
	}

	ipConf := nic.Properties.IPConfigurations[0]
	publicIPID := *ipConf.Properties.PublicIPAddress.ID

	ipParts := strings.Split(publicIPID, "/")
	ipRG := ipParts[4]
	ipName := ipParts[8]

	pubIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create public IP client: %v", err)
	}

	pubIP, err := pubIPClient.Get(ctx, ipRG, ipName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %v", err)
	}

	if pubIP.Properties == nil || pubIP.Properties.IPAddress == nil {
		return "", fmt.Errorf("no public IP found")
	}

	publicIP := *pubIP.Properties.IPAddress
	return publicIP, nil
}
