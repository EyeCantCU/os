package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/spf13/cobra"

	"chainguard.dev/wolfi-vm/tools/go/stalemate/awspub"
	"chainguard.dev/wolfi-vm/tools/go/stalemate/cmd"
	"chainguard.dev/wolfi-vm/tools/go/stalemate/marketplace"
)

var (
	staleAfterDays int
	outputFormat   string
)

var rootCmd = &cobra.Command{
	Use:   "stalemate <awspub-directory>",
	Short: "Find stale AWS Marketplace images",
	Long: `A tool to scan awspub configuration and AWS Marketplace offerings to find:
- Products that have not been published to AWS Marketplace (orphaned)
- Published products that have not been updated in N days (stale)`,
	Args: cobra.ExactArgs(1),
	Run:  run,
}

func init() {
	rootCmd.Flags().IntVar(&staleAfterDays, "stale-after-days", 0, "Consider products older than N days stale (0 = images never go stale)")
	rootCmd.Flags().StringVar(&outputFormat, "output", "text", "Output format: json")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(command *cobra.Command, args []string) {
	awspubDir := args[0]

	if outputFormat != "json" {
		log.Fatalf("Invalid output format: %s (must be 'json')", outputFormat)
	}

	if _, err := os.Stat(awspubDir); err != nil {
		log.Fatalf("Cannot find awspub directory at %s", awspubDir)
	}

	productMap, err := awspub.ProductsFromDir(awspubDir)
	if err != nil {
		log.Fatalf("Error scanning awspub directory: %v", err)
	}

	ctx := context.Background()
	marketplaceClient, err := marketplace.NewClient(ctx)
	if err != nil {
		log.Fatalf("Cannot create AWS Marketplace client: %v", err)
	}

	data, err := cmd.FindStaleProducts(productMap, marketplaceClient, staleAfterDays)
	if err != nil {
		log.Fatalf("Error finding stale products: %v", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")
	encoder.Encode(data)
}
