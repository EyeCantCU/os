package cmd

import (
	"log"

	azruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/cobra"
)

func terminateCmd() *cobra.Command {
	var resourceGroup string
	var tagName string
	var subscriptionID string

	cmd := &cobra.Command{
		Use:   "terminate",
		Short: "Terminate Azure VMs by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to get Azure credentials: %v", err)
			}

			// --- VMs ---
			vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create VM client: %v", err)
			}

			var vmPollers []*azruntime.Poller[armcompute.VirtualMachinesClientDeleteResponse]
			var vmNames []string

			vmPager := vmClient.NewListPager(resourceGroup, nil)
			for vmPager.More() {
				page, err := vmPager.NextPage(ctx)
				if err != nil {
					log.Fatalf("failed to list VMs: %v", err)
				}
				for _, vm := range page.Value {
					if vm.Tags != nil {
						if _, ok := vm.Tags[tagName]; ok {
							log.Printf("Deleting VM: %s", *vm.Name)
							poller, err := vmClient.BeginDelete(ctx, resourceGroup, *vm.Name, nil)
							if err != nil {
								log.Printf("failed to delete VM %s: %v", *vm.Name, err)
								continue
							}
							vmPollers = append(vmPollers, poller)
							vmNames = append(vmNames, *vm.Name)
						}
					}
				}
			}

			for i, poller := range vmPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for VM %s deletion: %v", vmNames[i], err)
				} else {
					log.Printf("Deleted VM: %s", vmNames[i])
				}
			}

			// --- Disks ---
			diskClient, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create disk client: %v", err)
			}

			var diskPollers []*azruntime.Poller[armcompute.DisksClientDeleteResponse]
			var diskNames []string

			diskPager := diskClient.NewListByResourceGroupPager(resourceGroup, nil)
			for diskPager.More() {
				page, err := diskPager.NextPage(ctx)
				if err != nil {
					log.Fatalf("failed to list disks: %v", err)
				}
				for _, disk := range page.Value {
					if disk.Tags != nil {
						if _, ok := disk.Tags[tagName]; ok {
							log.Printf("Deleting disk: %s", *disk.Name)
							poller, err := diskClient.BeginDelete(ctx, resourceGroup, *disk.Name, nil)
							if err != nil {
								log.Printf("failed to delete disk %s: %v", *disk.Name, err)
								continue
							}
							diskPollers = append(diskPollers, poller)
							diskNames = append(diskNames, *disk.Name)
						}
					}
				}
			}

			for i, poller := range diskPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for disk %s deletion: %v", diskNames[i], err)
				} else {
					log.Printf("Deleted disk: %s", diskNames[i])
				}
			}

			// --- NICs ---
			nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create NIC client: %v", err)
			}

			var nicPollers []*azruntime.Poller[armnetwork.InterfacesClientDeleteResponse]
			var nicNames []string

			nicPager := nicClient.NewListPager(resourceGroup, nil)
			for nicPager.More() {
				page, err := nicPager.NextPage(ctx)
				if err != nil {
					log.Fatalf("failed to list NICs: %v", err)
				}
				for _, nic := range page.Value {
					if nic.Tags != nil {
						if _, ok := nic.Tags[tagName]; ok {
							log.Printf("Deleting NIC: %s", *nic.Name)
							poller, err := nicClient.BeginDelete(ctx, resourceGroup, *nic.Name, nil)
							if err != nil {
								log.Printf("failed to delete NIC %s: %v", *nic.Name, err)
								continue
							}
							nicPollers = append(nicPollers, poller)
							nicNames = append(nicNames, *nic.Name)
						}
					}
				}
			}

			for i, poller := range nicPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for NIC %s deletion: %v", nicNames[i], err)
				} else {
					log.Printf("Deleted NIC: %s", nicNames[i])
				}
			}

			// --- Public IPs ---
			ipClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create public IP client: %v", err)
			}

			var ipPollers []*azruntime.Poller[armnetwork.PublicIPAddressesClientDeleteResponse]
			var ipNames []string

			ipPager := ipClient.NewListPager(resourceGroup, nil)
			for ipPager.More() {
				page, err := ipPager.NextPage(ctx)
				if err != nil {
					log.Fatalf("failed to list public IPs: %v", err)
				}
				for _, ip := range page.Value {
					if ip.Tags != nil {
						if _, ok := ip.Tags[tagName]; ok {
							log.Printf("Deleting public IP: %s", *ip.Name)
							poller, err := ipClient.BeginDelete(ctx, resourceGroup, *ip.Name, nil)
							if err != nil {
								log.Printf("failed to delete public IP %s: %v", *ip.Name, err)
								continue
							}
							ipPollers = append(ipPollers, poller)
							ipNames = append(ipNames, *ip.Name)
						}
					}
				}
			}

			for i, poller := range ipPollers {
				_, err := poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("error waiting for public IP %s deletion: %v", ipNames[i], err)
				} else {
					log.Printf("Deleted public IP: %s", ipNames[i])
				}
			}
		},
	}

	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Resource group containing the VM(s) (required)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag key to identify VM(s) to terminate (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.MarkFlagRequired("subscription-id")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")

	return cmd
}
