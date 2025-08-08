package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// SeedPackage represents a manual seed for archive exclusion
type SeedPackage struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// SeedsFile represents the archive-seeds.json structure
type SeedsFile struct {
	Seeds    []SeedPackage `json:"seeds"`
	Metadata struct {
		Created     string `json:"created"`
		Description string `json:"description"`
		Version     string `json:"version"`
	} `json:"metadata"`
}

func seedDependenciesCmd() *cobra.Command {
	var (
		seedsFile    string
		arch         string
		useWithdrawn bool
		withdrawnDir string
	)

	cmd := &cobra.Command{
		Use:   "seed-dependencies",
		Short: "Resolve dependencies for manual seed packages",
		Long: `This command reads archive-seeds.json file, generates minimal APKO configurations
for each seed package, and resolves their full dependency chains. The results are written
to JSON files in the resolved/seeds/ directory for use by the archive command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return seedDependencies(cmd.Context(), seedsFile, arch, useWithdrawn, withdrawnDir)
		},
	}

	cmd.Flags().StringVar(&seedsFile, "seeds-file", "archive-seeds.json", "Path to archive-seeds.json file")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files for testing")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn indexes")

	return cmd
}

func seedDependencies(ctx context.Context, seedsFile, arch string, useWithdrawn bool, withdrawnDir string) error {
	log.Printf("Processing seed dependencies from %s for architecture %s", seedsFile, arch)

	// Read seeds file
	seedsData, err := readSeedsFile(seedsFile)
	if err != nil {
		return fmt.Errorf("reading seeds file: %w", err)
	}

	log.Printf("Found %d seed packages", len(seedsData.Seeds))

	// Determine output directories
	resolvedDir := "resolved"
	unresolvedDir := "unresolved"
	if useWithdrawn {
		resolvedDir = "withdrawn-test/resolved"
		unresolvedDir = "withdrawn-test/unresolved"
	}

	// Create output directories
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("creating resolved directory: %w", err)
	}
	if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
		return fmt.Errorf("creating unresolved directory: %w", err)
	}

	// Create subdirectory for detailed seed info
	seedsDetailDir := filepath.Join(resolvedDir, "seeds")
	if err := os.MkdirAll(seedsDetailDir, 0755); err != nil {
		return fmt.Errorf("creating seeds detail directory: %w", err)
	}

	outputFile := filepath.Join(resolvedDir, "seeds.json")
	unresolvedFile := filepath.Join(unresolvedDir, "seeds.json")

	// Build repository list
	var buildRepos []string
	if useWithdrawn {
		withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
		buildRepos = []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]}
		log.Printf("Using withdrawn indexes from %s", withdrawnDir)
	} else {
		buildRepos = []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]}
	}

	cache := apk.NewCache(true)

	// Collect all unique APK packages across all seeds
	allPackages := make(map[string]bool) // package -> true if used
	unresolvedSeeds := make([]struct {
		Package string `json:"package"`
		Version string `json:"version"`
		Error   string `json:"error"`
	}, 0)
	var mu sync.Mutex

	log.Printf("Resolving seed dependencies...")

	// Process seeds in parallel
	var g errgroup.Group
	g.SetLimit(runtime.GOMAXPROCS(0))

	for _, seed := range seedsData.Seeds {
		g.Go(func() error {
			// Capture variables for closure
			currentSeed := seed

			log.Printf("Resolving dependencies for seed: %s=%s", currentSeed.Package, currentSeed.Version)

			// Create minimal APKO configuration for this seed
			cfg := &apko_types.ImageConfiguration{
				Contents: apko_types.ImageContents{
					Packages: []string{fmt.Sprintf("%s=%s", currentSeed.Package, currentSeed.Version)},
				},
				Archs: []apko_types.Architecture{apko_types.Architecture(arch)},
			}

			// Resolve seed to APK packages
			packages, err := lockImageDependencies(ctx, cfg, cache, buildRepos, arch)
			if err != nil {
				log.Printf("Error resolving seed dependencies for %s=%s: %v", currentSeed.Package, currentSeed.Version, err)

				// Add to unresolved seeds list
				mu.Lock()
				unresolvedSeeds = append(unresolvedSeeds, struct {
					Package string `json:"package"`
					Version string `json:"version"`
					Error   string `json:"error"`
				}{
					Package: currentSeed.Package,
					Version: currentSeed.Version,
					Error:   err.Error(),
				})
				mu.Unlock()

				return nil // Don't fail the entire operation for one seed
			}

			// Save individual seed dependencies to detailed JSON file
			sortedPkgs := make([]string, len(packages))
			copy(sortedPkgs, packages)
			sort.Strings(sortedPkgs)

			seedDetailFile := filepath.Join(seedsDetailDir, fmt.Sprintf("%s_%s.json", currentSeed.Package, currentSeed.Version))
			seedData := struct {
				SeedPackage  string   `json:"seed_package"`
				Reason       string   `json:"reason"`
				Architecture string   `json:"architecture"`
				Dependencies []string `json:"dependencies"`
			}{
				SeedPackage:  fmt.Sprintf("%s=%s", currentSeed.Package, currentSeed.Version),
				Reason:       currentSeed.Reason,
				Architecture: arch,
				Dependencies: sortedPkgs,
			}

			if err := func() error {
				file, err := os.Create(seedDetailFile)
				if err != nil {
					return err
				}
				defer file.Close()

				encoder := json.NewEncoder(file)
				encoder.SetIndent("", "  ")
				return encoder.Encode(seedData)
			}(); err != nil {
				log.Printf("Warning: failed to write detailed dependencies for %s=%s: %v", currentSeed.Package, currentSeed.Version, err)
			}

			// Add all packages to the set (with mutex protection)
			mu.Lock()
			for _, pkg := range packages {
				allPackages[pkg] = true
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error processing seed dependencies: %w", err)
	}

	// Convert set to sorted slice
	packageList := make([]string, 0, len(allPackages))
	for pkg := range allPackages {
		packageList = append(packageList, pkg)
	}
	sort.Strings(packageList)

	// Create JSON structure
	data := struct {
		Architecture      string   `json:"architecture"`
		SeedDependencies  []string `json:"seed_dependencies"`
		SeedCount         int      `json:"seed_count"`
		TotalDependencies int      `json:"total_dependencies"`
	}{
		Architecture:      arch,
		SeedDependencies:  packageList,
		SeedCount:         len(seedsData.Seeds),
		TotalDependencies: len(packageList),
	}

	// Write to JSON file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", outputFile, err)
	}
	defer file.Close()

	log.Printf("Writing %d seed dependencies from %d seeds to %s", len(packageList), len(seedsData.Seeds), outputFile)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
	}

	log.Printf("Successfully wrote seed dependencies to %s", outputFile)

	// Write unresolved seeds to JSON file if there are any
	if len(unresolvedSeeds) > 0 {
		unresolvedData := struct {
			Architecture    string `json:"architecture"`
			UnresolvedSeeds []struct {
				Package string `json:"package"`
				Version string `json:"version"`
				Error   string `json:"error"`
			} `json:"unresolved_seeds"`
		}{
			Architecture:    arch,
			UnresolvedSeeds: unresolvedSeeds,
		}

		unresolvedFile, err := os.Create(unresolvedFile)
		if err != nil {
			return fmt.Errorf("creating unresolved file %s: %w", unresolvedFile, err)
		}
		defer unresolvedFile.Close()

		log.Printf("Writing %d unresolved seeds to %s", len(unresolvedSeeds), unresolvedFile.Name())

		unresolvedEncoder := json.NewEncoder(unresolvedFile)
		unresolvedEncoder.SetIndent("", "  ")
		if err := unresolvedEncoder.Encode(unresolvedData); err != nil {
			return fmt.Errorf("encoding unresolved JSON: %w", err)
		}

		log.Printf("Successfully wrote unresolved seeds to %s", unresolvedFile.Name())
	}

	return nil
}

func readSeedsFile(filename string) (*SeedsFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filename, err)
	}

	var seedsFile SeedsFile
	if err := json.Unmarshal(data, &seedsFile); err != nil {
		return nil, fmt.Errorf("parsing JSON from %s: %w", filename, err)
	}

	// Validate seeds
	for i, seed := range seedsFile.Seeds {
		if seed.Package == "" {
			return nil, fmt.Errorf("seed %d: package name is required", i)
		}
		if seed.Version == "" {
			return nil, fmt.Errorf("seed %d: version is required", i)
		}
		if seed.Reason == "" {
			return nil, fmt.Errorf("seed %d: reason is required", i)
		}
	}

	return &seedsFile, nil
}
