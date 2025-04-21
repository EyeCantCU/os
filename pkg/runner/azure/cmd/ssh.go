package cmd

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"chainguard.dev/wolfi-vm/vm-test/pkg/internal/utils/sshutils"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/cobra"
)

func sshCmd() *cobra.Command {
	var (
		resourceGroup  string
		vmTag          string
		subscriptionID string
		privateKeyPath string
		sshUser        string
	)

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into an Azure VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, subscriptionID, resourceGroup, vmTag)
			log.Printf("Connecting to VM at %s", publicIP)

			if err := sshutils.SSHToInstance(ctx, publicIP, privateKeyPath, sshUser, args); err != nil {
				log.Fatalf("SSH error: %v", err)
			}

		},
	}

	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&vmTag, "tag", "", "Tag name of the VM (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVar(&sshUser, "user", "azureuser", "SSH username")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "id_ed25519", "Path to private SSH key")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("subscription")

	return cmd
}

func runRemoteCmd() *cobra.Command {
	var (
		resourceGroup  string
		vmTag          string
		subscriptionID string
		privateKeyPath string
		sshUser        string
		localFilePath  string
	)

	cmd := &cobra.Command{
		Use:   "run-remote",
		Short: "run a binary on an Azure VM by tag",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			publicIP := getIPByTag(ctx, subscriptionID, resourceGroup, vmTag)
			log.Printf("Connecting to VM at %s", publicIP)

			remoteFile := filepath.Join("/tmp", filepath.Base(localFilePath))

			err := sshutils.ShoveBinaryFile(ctx, publicIP, privateKeyPath, sshUser, localFilePath, remoteFile)
			if err != nil {
				log.Fatalf("failed to move %s to remote host: %v", localFilePath, err)
			}

			remotecmd := append([]string{"exec", remoteFile}, args...)

			if err := sshutils.SSHToInstance(ctx, publicIP, privateKeyPath, sshUser, remotecmd); err != nil {
				log.Fatalf("failed to run %s on remote host: %v", filepath.Base(localFilePath), err)
			}
		},
	}

	cmd.Flags().StringVar(&localFilePath, "file", "", "File to run remotely (required)")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required)")
	cmd.Flags().StringVar(&vmTag, "tag", "", "Tag name of the VM (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (required)")
	cmd.Flags().StringVar(&sshUser, "user", "azureuser", "SSH username")
	cmd.Flags().StringVar(&privateKeyPath, "private-key", "id_ed25519", "Path to private SSH key")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("subscription")

	return cmd
}

func getIPByTag(ctx context.Context, subscriptionID, resourceGroup, vmTag string) string {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to get Azure credentials: %v", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("failed to create VM client: %v", err)
	}

	pager := vmClient.NewListPager(resourceGroup, nil)

	var (
		vmName string
		nicID  string
	)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatalf("failed to get VM list: %v", err)
		}
		for _, vm := range page.Value {
			if vm.Tags != nil {
				if _, ok := vm.Tags[vmTag]; ok {
					vmName = *vm.Name
					nicID = *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID
					break
				}
			}
		}
		if vmName != "" {
			break
		}
	}

	if vmName == "" {
		log.Fatalf("no VM found with tag '%s'", vmTag)
	}

	parts := strings.Split(nicID, "/")
	if len(parts) < 9 {
		log.Fatalf("unexpected NIC ID format: %s", nicID)
	}
	nicRG := parts[4]
	nicName := parts[8]

	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("failed to create NIC client: %v", err)
	}

	nic, err := nicClient.Get(ctx, nicRG, nicName, nil)
	if err != nil {
		log.Fatalf("failed to get NIC: %v", err)
	}

	if nic.Properties == nil || nic.Properties.IPConfigurations == nil || len(nic.Properties.IPConfigurations) == 0 {
		log.Fatalf("NIC has no IP configuration")
	}

	ipConf := nic.Properties.IPConfigurations[0]
	publicIPID := *ipConf.Properties.PublicIPAddress.ID

	ipParts := strings.Split(publicIPID, "/")
	ipRG := ipParts[4]
	ipName := ipParts[8]

	pubIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		log.Fatalf("failed to create public IP client: %v", err)
	}

	pubIP, err := pubIPClient.Get(ctx, ipRG, ipName, nil)
	if err != nil {
		log.Fatalf("failed to get public IP: %v", err)
	}

	if pubIP.Properties == nil || pubIP.Properties.IPAddress == nil {
		log.Fatalf("no public IP found")
	}

	publicIP := *pubIP.Properties.IPAddress
	return publicIP
}
