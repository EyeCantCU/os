package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

func sshToInstance(host, privateKeyPath, user string, command []string) error {
	key, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	if len(command) > 0 {
		cmdString := strings.Join(command, " ")
		output, err := session.CombinedOutput(cmdString)
		if err != nil {
			return fmt.Errorf("command error: %w\nOutput: %s", err, output)
		}
		fmt.Print(string(output))
	} else {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		session.Stdin = os.Stdin
		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

		term := os.Getenv("TERM")
		if term == "" {
			term = "xterm"
		}

		if err := session.RequestPty(term, 80, 40, modes); err != nil {
			return fmt.Errorf("request for pseudo terminal failed: %w", err)
		}

		if err := session.Shell(); err != nil {
			return fmt.Errorf("failed to start shell: %w", err)
		}
		session.Wait()
	}

	return nil
}

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
			ctx := context.Background()

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
				log.Fatal("no VM found with tag '%s'", vmTag)
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
			log.Printf("Connecting to VM at %s", publicIP)

			if err := sshToInstance(publicIP, privateKeyPath, sshUser, args); err != nil {
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

