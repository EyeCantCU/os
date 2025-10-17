package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

type ResourceType string

const (
	ResourceTypeAll          ResourceType = "all"
	ResourceTypeDisk         ResourceType = "disk"
	ResourceTypeImage        ResourceType = "image"
	ResourceTypeImageVersion ResourceType = "imageversion"
)

var validResourceTypes = []ResourceType{
	ResourceTypeAll,
	ResourceTypeDisk,
	ResourceTypeImage,
	ResourceTypeImageVersion,
}

// Deletion sets define the order of resource deletion
// Each set can be deleted in parallel, but sets must be processed sequentially
var deletionSets = [][]ResourceType{
	// Set 1: Delete Disks and Image Versions first
	{
		ResourceTypeDisk,
		ResourceTypeImageVersion,
	},
	// Set 2: Delete dependent resources
	{
		ResourceTypeImage,
	},
}

type ResourceInfo struct {
	ID           string
	Name         string
	Type         string
	CreatedTime  *time.Time
	Tags         map[string]*string
	ResourceType ResourceType
}

type DeletionResult struct {
	Resource      *ResourceInfo
	Success       bool
	Error         error
	RetryAttempts int
}

func getAllResourcesInGroup(ctx context.Context, clients *AzureClients, resourceGroup string) ([]*ResourceInfo, error) {
	var allResources []*ResourceInfo

	// Get Disks
	if shouldIncludeResourceType(ResourceTypeDisk) {
		resources, err := getDisks(ctx, clients, resourceGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to get disks: %w", err)
		}
		allResources = append(allResources, resources...)
	}

	// Get Images
	if shouldIncludeResourceType(ResourceTypeImage) {
		resources, err := getImages(ctx, clients, resourceGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to get images: %w", err)
		}
		allResources = append(allResources, resources...)
	}

	// Get Gallery Image Versions
	if shouldIncludeResourceType(ResourceTypeImageVersion) {
		resources, err := getImageVersions(ctx, clients, resourceGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to get gallery image versions: %w", err)
		}
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

func shouldIncludeResourceType(resourceType ResourceType) bool {
	return slices.Contains(resourceTypes, string(ResourceTypeAll)) || slices.Contains(resourceTypes, string(resourceType))
}

func deleteResource(ctx context.Context, clients *AzureClients, resource *ResourceInfo) error {
	switch resource.ResourceType {
	case ResourceTypeDisk:
		return deleteDisk(ctx, clients.Disks, resource.Name)
	case ResourceTypeImage:
		return deleteImage(ctx, clients.Images, resource.Name)
	case ResourceTypeImageVersion:
		return deleteImageVersion(ctx, clients, resource)
	default:
		return fmt.Errorf("unsupported resource type for deletion: %s", resource.ResourceType)
	}
}

func isValidResourceType(rt ResourceType) bool {
	return slices.Contains(validResourceTypes, rt)
}

func shouldCleanupResourceType(actualType ResourceType, allowedTypes []string) bool {
	if slices.Contains(allowedTypes, string(ResourceTypeAll)) {
		return true
	}

	for _, allowedType := range allowedTypes {
		if string(actualType) == allowedType {
			return true
		}
	}

	return false
}

func hasProtectionTag(tags map[string]*string, protectionTag string) bool {
	if tags == nil {
		return false
	}

	for tagName := range tags {
		if tagName == protectionTag {
			return true
		}
	}

	return false
}

// filterResourcesForDeletion applies all filtering logic to identify resources for deletion
func filterResourcesForDeletion(resources []*ResourceInfo, nameRegex, excludeNameRegex *regexp.Regexp, maxAge time.Duration, verbose bool) []*ResourceInfo {
	var filtered []*ResourceInfo
	cutoffTime := time.Now().Add(-maxAge)

	for _, resource := range resources {
		// Check name pattern filters
		if nameRegex != nil && !nameRegex.MatchString(resource.Name) {
			if verbose {
				log.Printf("Resource %s (%s) filtered out by name pattern", resource.Name, resource.Type)
			}
			continue
		}
		if excludeNameRegex != nil && excludeNameRegex.MatchString(resource.Name) {
			if verbose {
				log.Printf("Resource %s (%s) filtered out by exclude name pattern", resource.Name, resource.Type)
			}
			continue
		}

		// Check resource type filter
		if !shouldCleanupResourceType(resource.ResourceType, resourceTypes) {
			if verbose {
				log.Printf("Resource %s (%s) filtered out by resource type", resource.Name, resource.Type)
			}
			continue
		}

		// Check protection tag
		if protectionTag != "" && hasProtectionTag(resource.Tags, protectionTag) {
			if verbose {
				log.Printf("Resource %s (%s) protected by tag %s", resource.Name, resource.Type, protectionTag)
			}
			continue
		}

		// Check age
		if resource.CreatedTime == nil {
			if verbose {
				log.Printf("Cannot determine creation time for %s (%s): no timestamp available", resource.Name, resource.Type)
			}
			continue
		}

		if resource.CreatedTime.After(cutoffTime) {
			if verbose {
				age := time.Since(*resource.CreatedTime)
				log.Printf("Resource %s (%s) too young: %v old", resource.Name, resource.Type, age.Round(time.Minute))
			}
			continue
		}

		// Resource matches deletion criteria
		age := time.Since(*resource.CreatedTime)
		log.Printf("Resource %s (%s) marked for deletion: %v old", resource.Name, resource.Type, age.Round(time.Minute))
		filtered = append(filtered, resource)
	}

	return filtered
}

// deleteResourcesInSets deletes resources in the proper dependency order using sets
func deleteResourcesInSets(ctx context.Context, clients *AzureClients, resources []*ResourceInfo, attempts int, verbose, dryRun bool) (int, []error) {
	var totalDeleted int
	var allErrors []error

	// Group resources by type for each deletion set
	resourcesByType := make(map[ResourceType][]*ResourceInfo)
	for _, resource := range resources {
		resourcesByType[resource.ResourceType] = append(resourcesByType[resource.ResourceType], resource)
	}

	// Process each deletion set sequentially
	for setIndex, deletionSet := range deletionSets {
		if verbose {
			log.Printf("Processing deletion set %d: %v", setIndex+1, deletionSet)
		}

		var setResources []*ResourceInfo
		for _, resourceType := range deletionSet {
			if typeResources, exists := resourcesByType[resourceType]; exists {
				setResources = append(setResources, typeResources...)
			}
		}

		if len(setResources) == 0 {
			if verbose {
				log.Printf("No resources in set %d to delete", setIndex+1)
			}
			continue
		}

		// Delete resources in this set in parallel
		deleted, errors := deleteResourcesInParallel(ctx, clients, setResources, attempts, verbose, dryRun)
		totalDeleted += deleted
		allErrors = append(allErrors, errors...)

		// Add a brief pause between sets to allow Azure to process dependencies
		if !dryRun && setIndex < len(deletionSets)-1 {
			time.Sleep(5 * time.Second)
		}
	}

	return totalDeleted, allErrors
}

// deleteResourcesInParallel deletes a slice of resources in parallel
func deleteResourcesInParallel(ctx context.Context, clients *AzureClients, resources []*ResourceInfo, attempts int, verbose, dryRun bool) (int, []error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var deleted int
	var errors []error

	for _, resource := range resources {
		wg.Add(1)
		go func(res *ResourceInfo) {
			defer wg.Done()

			if dryRun {
				log.Printf("DRY RUN: Would delete %s (%s)", res.Name, res.Type)
				mu.Lock()
				deleted++
				mu.Unlock()
				return
			}

			log.Printf("Deleting %s (%s)...", res.Name, res.Type)

			if err := deleteResourceWithRetry(ctx, clients, res, attempts); err != nil {
				log.Printf("Failed to delete %s (%s): %v", res.Name, res.Type, err)
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to delete %s: %w", res.Name, err))
				mu.Unlock()
			} else {
				log.Printf("Successfully deleted %s (%s)", res.Name, res.Type)
				mu.Lock()
				deleted++
				mu.Unlock()
			}
		}(resource)
	}

	wg.Wait()
	return deleted, errors
}

// deleteResourceWithRetry attempts to delete a resource with retries for transient failures
func deleteResourceWithRetry(ctx context.Context, clients *AzureClients, resource *ResourceInfo, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := deleteResource(ctx, clients, resource)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if this is a retryable error (dependency conflicts, throttling, etc.)
		if isRetryableError(err) && attempt < maxRetries {
			waitTime := time.Duration(attempt) * 10 * time.Second
			log.Printf("Delete attempt %d failed for %s, retrying in %v: %v", attempt, resource.Name, waitTime, err)
			time.Sleep(waitTime)
			continue
		}

		break
	}

	return lastErr
}

// isRetryableError determines if an error is worth retrying
func isRetryableError(err error) bool {
	errStr := err.Error()
	retryableErrors := []string{
		"throttled",
		"TooManyRequests",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}

func unixZero() *time.Time {
	t := time.Unix(0, 0)
	return &t
}
