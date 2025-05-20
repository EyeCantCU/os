package cmd

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"chainguard.dev/wolfi-vm/vm-test/pkg/runner/azure/azutil"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/spf13/cobra"
)

func consoleLogCmd() *cobra.Command {
	var (
		vmTag          string
		resourceGroup  string
		subscriptionID string
		wait           bool
	)

	cmd := &cobra.Command{
		Use:   "console-log",
		Short: "write the console log to stdout",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			cred, err := azidentity.NewDefaultAzureCredential(nil)
			if err != nil {
				log.Fatalf("failed to obtain Azure credential: %v", err)
			}

			vm, err := azutil.GetFirstInstanceByTag(ctx, cred, subscriptionID, resourceGroup, vmTag)
			if err != nil {
				log.Fatalf("could not find instance: %v", err)
			}

			vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
			if err != nil {
				log.Fatalf("failed to create VM client: %v", err)
			}

			diagnosticsResp, err := vmClient.RetrieveBootDiagnosticsData(ctx, resourceGroup, *vm.Name, nil)
			if err != nil {
				log.Fatalf("could not get boot diagnostics data: %v", err)
			}

			if diagnosticsResp.SerialConsoleLogBlobURI == nil {
				log.Fatalf("boot diagnostics console log URI is nil for vm %s", *vm.Name)
			}

			var consoleResp *http.Response
			log.Printf("fetching console from %s", *vm.Name)
			for {
				consoleResp, err = http.Get(*diagnosticsResp.SerialConsoleLogBlobURI)
				if err != nil {
					log.Fatalf("could get get serial console log: %v", err)
				}
				if consoleResp.StatusCode == 200 {
					break
				}
				if !wait {
					log.Fatalf("serial console log returned status %v", consoleResp.Status)
				}
				select {
				case <-ctx.Done():
					wait = false
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}
			_, err = io.Copy(os.Stdout, consoleResp.Body)
			if err != nil {
				log.Fatalf("error printing response: %v", err)
			}
		},
	}

	cmd.Flags().StringVar(&vmTag, "tag", "", "VM instance tag (required)")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure Resource Group name (required)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure Subscription ID (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for serial to become available")
	cmd.MarkFlagRequired("tag")
	cmd.MarkFlagRequired("resource-group")
	cmd.MarkFlagRequired("subscription-id")

	return cmd
}
