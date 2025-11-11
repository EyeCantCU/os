package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/spf13/cobra"
)

var (
	subscriptionID     string
	resourceGroup      string
	resourceTypes      []string
	maxAge             time.Duration
	protectionTag      string
	dryRun             bool
	verbose            bool
	namePattern        string
	excludeNamePattern string
	retryAttempts      int
	jobs               int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "azure-cleanup",
		Short: "Clean up Azure resources based on age and filtering policies with dependency-aware deletion",
		Long: `Clean up Azure resources based on age and filtering policies.

This tool deletes Azure resources in dependency-aware batches to handle resource relationships.

Examples:
  # Dry run to see what would be deleted (recommended first step)
  azure-cleanup --subscription-id "abc123" --resource-group "test-rg" --max-age 24h --dry-run

  # Clean up all resources older than 7 days, protecting anything with the "do-not-delete" tag
  azure-cleanup --subscription-id "abc123" --resource-group "test-rg" \
                --max-age 168h --protection-tag "do-not-delete"

  # Clean up only VMs and disks older than 1 day
  azure-cleanup --subscription-id "abc123" --resource-group "test-rg" \
                --max-age 24h --resource-types "vm,disk"

  # Clean up resources matching a name pattern
  azure-cleanup --subscription-id "abc123" --resource-group "test-rg" \
                --max-age 24h --name-pattern "test-.*"

  # Limit parallel deletions to 5 jobs at once
  azure-cleanup --subscription-id "abc123" --resource-group "test-rg" \
                --max-age 24h --resource-types "routetable,nsg" --jobs 5`,
		RunE: run,
	}

	rootCmd.Flags().StringVarP(&subscriptionID, "subscription-id", "s", "", "Azure subscription ID")
	rootCmd.Flags().StringVarP(&resourceGroup, "resource-group", "g", "", "Resource group to clean up")
	rootCmd.Flags().StringSliceVarP(&resourceTypes, "resource-types", "t", []string{"all"}, "Comma-separated list of resource types to clean up (all,disk,image,imageversion,vm,nic,ip,vnet,routetable,nsg)")
	rootCmd.Flags().DurationVarP(&maxAge, "max-age", "a", 24*time.Hour, "Maximum age of resources to keep (e.g., 24h, 7d)")
	rootCmd.Flags().StringVarP(&protectionTag, "protection-tag", "p", "do-not-delete", "Tag name that prevents deletion (resources with this tag will be skipped)")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be deleted without executing")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().IntVar(&retryAttempts, "retries", 3, "Allowed number of retries per-resource for deletion.")
	// Set at the refill rate for subscription level delete request throttling
	// https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/request-limits-and-throttling
	rootCmd.Flags().IntVar(&jobs, "jobs", 10, "Maximum number of parallel deletions to run at once")
	rootCmd.Flags().StringVar(&namePattern, "name-pattern", "", "Regular expression pattern for resource names to include")
	rootCmd.Flags().StringVar(&excludeNamePattern, "exclude-name-pattern", "", "Regular expression pattern for resource names to exclude")

	rootCmd.MarkFlagRequired("subscription-id")
	rootCmd.MarkFlagRequired("resource-group")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Validate resource types
	for _, rt := range resourceTypes {
		if !isValidResourceType(ResourceType(rt)) {
			return fmt.Errorf("invalid resource type: %s. Valid types: %v", rt, validResourceTypes)
		}
	}

	// Compile regex patterns if provided
	var nameRegex, excludeNameRegex *regexp.Regexp
	var err error
	if namePattern != "" {
		nameRegex, err = regexp.Compile(namePattern)
		if err != nil {
			return fmt.Errorf("invalid name pattern regex: %w", err)
		}
	}
	if excludeNamePattern != "" {
		excludeNameRegex, err = regexp.Compile(excludeNamePattern)
		if err != nil {
			return fmt.Errorf("invalid exclude name pattern regex: %w", err)
		}
	}

	ctx := context.Background()
	clients, err := createAzureClients(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create Azure clients: %w", err)
	}

	if verbose {
		log.Printf("Scanning resources in subscription %s, resource group %s", subscriptionID, resourceGroup)
		log.Printf("Resource types: %v", resourceTypes)
		log.Printf("Max age: %v", maxAge)
		if protectionTag != "" {
			log.Printf("Protection tag: %s", protectionTag)
		}
		if namePattern != "" {
			log.Printf("Name pattern: %s", namePattern)
		}
		if excludeNamePattern != "" {
			log.Printf("Exclude name pattern: %s", excludeNamePattern)
		}
	}

	resources, err := getAllResourcesInGroup(ctx, clients, resourceGroup)
	if err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	// Filter resources that match deletion criteria
	resourcesToDelete := filterResourcesForDeletion(resources, nameRegex, excludeNameRegex, maxAge, verbose)

	var protected, tooYoung, filtered int
	for _, resource := range resources {
		found := false
		for _, toDelete := range resourcesToDelete {
			if toDelete.ID == resource.ID {
				found = true
				break
			}
		}
		if !found {
			if protectionTag != "" && hasProtectionTag(resource.Tags, protectionTag) {
				protected++
			} else if !shouldCleanupResourceType(resource.ResourceType, resourceTypes) {
				filtered++
			} else if resource.CreatedTime != nil && resource.CreatedTime.After(time.Now().Add(-maxAge)) {
				tooYoung++
			} else {
				filtered++ // Other filtering reasons
			}
		}
	}

	// Delete resources in sets with proper ordering
	deleted, deleteErrors := deleteResourcesInSets(ctx, clients, resourcesToDelete, retryAttempts, jobs, verbose, dryRun)

	if dryRun {
		log.Printf("DRY RUN complete: %d resources would be deleted, %d protected, %d too young, %d filtered",
			deleted, protected, tooYoung, filtered)
	} else {
		log.Printf("Cleanup complete: %d resources deleted, %d protected, %d too young, %d filtered",
			deleted, protected, tooYoung, filtered)
	}

	if len(deleteErrors) > 0 {
		log.Printf("Cleanup completed with %d errors", len(deleteErrors))
		return fmt.Errorf("cleanup completed with %d errors", len(deleteErrors))
	}

	if deleted == 0 {
		log.Printf("No resources were deleted")
	}

	return nil
}
