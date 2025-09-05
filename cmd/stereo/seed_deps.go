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

// SeedOrigin represents a manual seed origin for archive exclusion
type SeedOrigin struct {
	Origin  string `json:"origin"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// SeedsFile represents the archive-seeds.json structure
type SeedsFile struct {
	Seeds    []SeedOrigin `json:"seeds"`
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
		Short: "Resolve dependencies for manual seed origins",
		Long: `This command reads archive-seeds.json file, finds all subpackages for each seed origin
with the specified version by searching APKINDEX files, and resolves their full dependency
chains. The results are written to JSON files in the resolved/seeds/ directory for use by
the archive command.

When no --arch is specified, dependencies are computed for both x86_64 and aarch64 architectures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return seedDependencies(cmd.Context(), seedsFile, architectures, useWithdrawn, withdrawnDir)
		},
	}

	cmd.Flags().StringVar(&seedsFile, "seeds-file", "archive-seeds.json", "Path to archive-seeds.json file")
	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files for testing")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn indexes")

	return cmd
}

func seedDependencies(ctx context.Context, seedsFile string, architectures []string, useWithdrawn bool, withdrawnDir string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Processing seed dependencies from %s for architectures %v using withdrawn indexes from %s...", seedsFile, architectures, withdrawnDir)
	} else {
		log.Printf("Processing seed dependencies from %s for architectures %v...", seedsFile, architectures)
	}

	// Read seeds file
	seedsData, err := readSeedsFile(seedsFile)
	if err != nil {
		return fmt.Errorf("reading seeds file: %w", err)
	}

	log.Printf("Found %d seed packages", len(seedsData.Seeds))

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories - use withdrawn-test prefix when testing with withdrawn indexes
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "seeds", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "seeds", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "seeds", arch)
			unresolvedDir = filepath.Join("unresolved", "seeds", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved seeds directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved seeds directory: %w", err)
		}

		// Create subdirectory for detailed seed info
		seedsDetailDir := filepath.Join(resolvedDir, "seeds")
		if err := os.MkdirAll(seedsDetailDir, 0755); err != nil {
			return fmt.Errorf("creating seeds detail directory %s: %w", seedsDetailDir, err)
		}

		outputFile := filepath.Join(resolvedDir, "seeds.json")
		unresolvedFile := filepath.Join(unresolvedDir, "seeds.json")

		// Build repository list
		var buildRepos []string
		if useWithdrawn {
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]}
		} else {
			buildRepos = []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]}
		}

		cache := apk.NewCache(true)

		// First, fetch APKINDEX for all repositories to find subpackages
		log.Printf("Loading APKINDEX files for architecture: %s...", arch)
		var allPackagesByOrigin = make(map[string]map[string][]*apk.Package) // origin -> version -> packages

		for _, repoURL := range buildRepos {
			var index *apk.APKIndex
			var err error

			if useWithdrawn {
				// Load from local file
				indexPath := filepath.Join(repoURL, arch, "APKINDEX.tar.gz")
				index, err = loadLocalAPKIndex(indexPath)
				if err != nil {
					log.Printf("Warning: Could not load local APKINDEX from %s: %v", indexPath, err)
					continue
				}
			} else {
				// Fetch from URL
				index, err = fetchAPKIndex(ctx, repoURL, arch)
				if err != nil {
					log.Printf("Warning: Could not fetch APKINDEX for %s (arch: %s): %v", repoURL, arch, err)
					continue
				}
			}

			// Group packages by origin and version
			for _, pkg := range index.Packages {
				if pkg.Origin == "" {
					continue // Skip packages without origin
				}
				if allPackagesByOrigin[pkg.Origin] == nil {
					allPackagesByOrigin[pkg.Origin] = make(map[string][]*apk.Package)
				}
				allPackagesByOrigin[pkg.Origin][pkg.Version] = append(allPackagesByOrigin[pkg.Origin][pkg.Version], pkg)
			}
		}

		// Collect all unique APK packages across all seeds
		allPackages := make(map[string]bool) // package -> true if used
		unresolvedSeeds := make([]struct {
			Origin  string `json:"origin"`
			Version string `json:"version"`
			Error   string `json:"error"`
		}, 0)
		var mu sync.Mutex

		log.Printf("Resolving seed dependencies for architecture: %s...", arch)

		// Process seeds in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for _, seed := range seedsData.Seeds {
			g.Go(func() error {
				// Capture variables for closure
				currentSeed := seed
				currentArch := arch

				log.Printf("Resolving dependencies for seed origin: %s=%s (architecture: %s)", currentSeed.Origin, currentSeed.Version, currentArch)

				// Find all subpackages for this origin and version
				subpackages, exists := allPackagesByOrigin[currentSeed.Origin][currentSeed.Version]
				if !exists || len(subpackages) == 0 {
					err := fmt.Errorf("no packages found for origin %s with version %s", currentSeed.Origin, currentSeed.Version)
					log.Printf("Error finding subpackages for origin %s=%s (architecture: %s): %v", currentSeed.Origin, currentSeed.Version, currentArch, err)

					// Add to unresolved seeds list
					mu.Lock()
					unresolvedSeeds = append(unresolvedSeeds, struct {
						Origin  string `json:"origin"`
						Version string `json:"version"`
						Error   string `json:"error"`
					}{
						Origin:  currentSeed.Origin,
						Version: currentSeed.Version,
						Error:   err.Error(),
					})
					mu.Unlock()
					return nil // Don't fail the entire operation
				}

				log.Printf("Found %d subpackages for origin %s=%s: %v", len(subpackages), currentSeed.Origin, currentSeed.Version,
					func() []string {
						names := make([]string, len(subpackages))
						for i, p := range subpackages {
							names[i] = p.Name
						}
						return names
					}())

				// Create package list for APKO configuration
				var packageList []string
				for _, pkg := range subpackages {
					packageList = append(packageList, fmt.Sprintf("%s=%s", pkg.Name, pkg.Version))
				}

				// Create minimal APKO configuration for all subpackages
				cfg := &apko_types.ImageConfiguration{
					Contents: apko_types.ImageContents{
						Packages: packageList,
					},
					Archs: []apko_types.Architecture{apko_types.Architecture(currentArch)},
				}

				// Resolve origin subpackages to APK packages
				packages, err := lockImageDependencies(ctx, cfg, cache, buildRepos, currentArch)
				if err != nil {
					log.Printf("Error resolving seed dependencies for origin %s=%s (architecture: %s): %v", currentSeed.Origin, currentSeed.Version, currentArch, err)

					// Add to unresolved seeds list
					mu.Lock()
					unresolvedSeeds = append(unresolvedSeeds, struct {
						Origin  string `json:"origin"`
						Version string `json:"version"`
						Error   string `json:"error"`
					}{
						Origin:  currentSeed.Origin,
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

				// Create subpackage names list
				subpackageNames := make([]string, len(subpackages))
				for i, pkg := range subpackages {
					subpackageNames[i] = pkg.Name
				}
				sort.Strings(subpackageNames)

				seedDetailFile := filepath.Join(seedsDetailDir, fmt.Sprintf("%s_%s.json", currentSeed.Origin, currentSeed.Version))
				seedData := struct {
					SeedOrigin   string   `json:"seed_origin"`
					SeedVersion  string   `json:"seed_version"`
					Subpackages  []string `json:"subpackages"`
					Reason       string   `json:"reason"`
					Architecture string   `json:"architecture"`
					Dependencies []string `json:"dependencies"`
				}{
					SeedOrigin:   currentSeed.Origin,
					SeedVersion:  currentSeed.Version,
					Subpackages:  subpackageNames,
					Reason:       currentSeed.Reason,
					Architecture: currentArch,
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
					log.Printf("Warning: failed to write detailed dependencies for origin %s=%s: %v", currentSeed.Origin, currentSeed.Version, err)
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
			return fmt.Errorf("error processing seed dependencies (architecture: %s): %w", arch, err)
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

		log.Printf("Successfully wrote seed dependencies for architecture %s to %s", arch, outputFile)

		// Write unresolved seeds to JSON file if there are any
		if len(unresolvedSeeds) > 0 {
			unresolvedData := struct {
				Architecture    string `json:"architecture"`
				UnresolvedSeeds []struct {
					Origin  string `json:"origin"`
					Version string `json:"version"`
					Error   string `json:"error"`
				} `json:"unresolved_seeds"`
			}{
				Architecture:    arch,
				UnresolvedSeeds: unresolvedSeeds,
			}

			unresolvedFileHandle, err := os.Create(unresolvedFile)
			if err != nil {
				log.Printf("Warning: Could not create unresolved seeds file %s: %v", unresolvedFile, err)
			} else {
				log.Printf("Writing %d unresolved seeds to %s", len(unresolvedSeeds), unresolvedFile)

				encoder := json.NewEncoder(unresolvedFileHandle)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(unresolvedData); err != nil {
					log.Printf("Warning: Error encoding unresolved seeds JSON to %s: %v", unresolvedFile, err)
				} else {
					log.Printf("Successfully wrote unresolved seeds for architecture %s to %s", arch, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("Seed dependency processing complete for architectures %v", architectures)
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
		if seed.Origin == "" {
			return nil, fmt.Errorf("seed %d: origin is required", i)
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
