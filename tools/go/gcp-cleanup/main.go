package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/GoogleCloudPlatform/cloud-image-tests/cleanerupper"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

var (
	project            string
	resourceTypes      []string
	maxAge             time.Duration
	protectionTag      string
	dryRun             bool
	verbose            bool
	namePattern        string
	excludeNamePattern string
	retryAttempts      int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gcp-clean",
		Short: "Clean up GCP resources based on age and filtering policies",
		RunE:  run,
	}

	rootCmd.Flags().StringVarP(&project, "project", "p", "", "GCP Project")
	rootCmd.Flags().StringSliceVarP(&resourceTypes, "resource-types", "t", []string{"all"}, "Comma-separated list of resource types to clean up (all,instances,images,disks,networks)")
	rootCmd.Flags().DurationVarP(&maxAge, "max-age", "a", 24*time.Hour, "Maximum age of resources to keep (e.g., 24h, 7d)")
	rootCmd.Flags().StringVar(&protectionTag, "protection-tag", "do-not-delete", "Tag name that prevents deletion (resources with this tag will be skipped)")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be deleted without executing")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().StringVar(&namePattern, "name-pattern", "", "Regular expression pattern for resource names to include")
	rootCmd.Flags().StringVar(&excludeNamePattern, "exclude-name-pattern", "", "Regular expression pattern for resource names to exclude")

	rootCmd.MarkFlagRequired("project")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx, compute.CloudPlatformScope)
	if err != nil {
		return err
	}

	clients, err := cleanerupper.NewClients(ctx, option.WithCredentials(creds))
	if err != nil {
		return err
	}

	policy, err := mkPolicyFromFlags()
	if err != nil {
		return err
	}

	if shouldClean("instances") {
		if verbose {
			log.Println("Cleaning instances")
		}
		insts, errs := cleanerupper.CleanInstances(*clients, project, policy, dryRun)
		for _, i := range insts {
			fmt.Printf(" - %s\n", i)
		}
		for _, e := range errs {
			fmt.Println(e)
		}
	}
	if shouldClean("disks") {
		if verbose {
			log.Println("Cleaning disks")
		}
		cleaned, errs := cleanerupper.CleanDisks(*clients, project, policy, dryRun)
		for _, c := range cleaned {
			fmt.Printf(" - %s\n", c)
		}
		for _, e := range errs {
			fmt.Println(e)
		}
	}
	if shouldClean("networks") {
		if verbose {
			log.Println("Cleaning networks")
		}
		cleaned, errs := cleanerupper.CleanNetworks(*clients, project, policy, dryRun)
		for _, c := range cleaned {
			fmt.Printf(" - %s\n", c)
		}
		for _, e := range errs {
			fmt.Println(e)
		}
	}
	if shouldClean("images") {
		if verbose {
			log.Println("Cleaning images")
		}
		cleaned, errs := cleanerupper.CleanImages(*clients, project, policy, dryRun)
		for _, c := range cleaned {
			fmt.Printf(" - %s\n", c)
		}
		for _, e := range errs {
			fmt.Println(e)
		}
	}

	return nil
}

func shouldClean(s string) bool {
	for _, i := range resourceTypes {
		if i == "all" || i == s {
			return true
		}
	}
	return false
}

func mkPolicyFromFlags() (cleanerupper.PolicyFunc, error) {
	namePatternRe, err := regexp.Compile(namePattern)
	if err != nil {
		return nil, err
	}
	excludeNamePatternRe, err := regexp.Compile(excludeNamePattern)
	if err != nil {
		return nil, err
	}

	t := time.Now().Add(time.Duration(-1) * maxAge)
	// This function decides what gets destroyed.
	// Forked from default AgePolicy at
	// https://github.com/GoogleCloudPlatform/cloud-image-tests/blob/main/cleanerupper/cleanerupper.go#L85
	return func(resource any) bool {
		var labels map[string]string
		var name string
		var created time.Time
		var err error
		switch r := resource.(type) {
		case *compute.Network:
			name = r.Name
			if r.Name == "default" {
				return false
			}
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.MachineImage:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Disk:
			name = r.Name
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Image:
			name = r.Name
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Snapshot:
			name = r.Name
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Instance:
			name = r.Name
			if r.DeletionProtection {
				return false
			}
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.ForwardingRule:
			labels = r.Labels
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.UrlMap:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.BackendService:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.TargetHttpProxy:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.HealthCheck:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.NetworkEndpointGroup:
			name = r.Name
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		default:
			return false
		}
		if err != nil {
			return false
		}
		if _, keep := labels[protectionTag]; keep {
			return false
		}

		if namePattern != "" && !namePatternRe.MatchString(name) {
			return false
		}

		if excludeNamePattern != "" && excludeNamePatternRe.MatchString(name) {
			return false
		}

		return t.After(created)
	}, nil
}
