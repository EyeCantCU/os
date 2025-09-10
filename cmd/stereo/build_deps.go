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
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func buildDepsCmd() *cobra.Command {
	var (
		arch         string
		useWithdrawn bool
		withdrawnDir string
	)

	cmd := &cobra.Command{
		Use:   "build-dependencies",
		Short: "Pre-compute build dependencies for all melange configurations",
		Long: `This command pre-computes and caches the build dependencies for all melange configurations
across the three repositories (os, extra-packages, enterprise-packages). The results are written
to files in the resolved/build/ directory and can be used by other commands to avoid expensive
dependency resolution. 

When no --arch is specified, dependencies are computed for both x86_64 and aarch64 architectures,
respecting any target-architecture constraints in melange configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return buildDeps(cmd.Context(), architectures, useWithdrawn, withdrawnDir)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")

	return cmd
}

// supportsArchitecture checks if a melange configuration supports the given architecture
func supportsArchitecture(cfg *config.Configuration, arch string) bool {
	// If no target-architecture is specified, package supports all architectures
	if len(cfg.Package.TargetArchitecture) == 0 {
		return true
	}

	// Check if the architecture is in the target-architecture list
	for _, targetArch := range cfg.Package.TargetArchitecture {
		if targetArch == arch {
			return true
		}
	}
	return false
}

func buildDeps(ctx context.Context, architectures []string, useWithdrawn bool, withdrawnDir string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Pre-computing build dependencies for architectures %v using withdrawn indexes from %s...", architectures, withdrawnDir)
	} else {
		log.Printf("Pre-computing build dependencies for architectures %v...", architectures)
	}

	// Get all melange configurations
	pkgss, err := dirToPackages(ctx)
	if err != nil {
		return fmt.Errorf("getting package configurations: %w", err)
	}

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories - use withdrawn-test prefix when testing with withdrawn indexes
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "build", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "build", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "build", arch)
			unresolvedDir = filepath.Join("unresolved", "build", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved build directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved build directory: %w", err)
		}

		// Create APK cache for build dependency resolution
		cache := apk.NewCache(true)

		// Build repository mapping for each directory
		var buildRepos map[string][]string
		if useWithdrawn {
			// Use local withdrawn indexes instead of remote repositories
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = map[string][]string{
				"os":                  []string{withdrawnRepos["os"]},
				"extra-packages":      []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"]},
				"enterprise-packages": []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]},
			}
		} else {
			// Use normal remote repositories
			buildRepos = map[string][]string{
				"os":                  []string{dirToRepo["os"]},
				"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
				"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
			}
		}

		// Process each repository directory
		for dir, pkgs := range pkgss {
			log.Printf("Processing build dependencies for repository: %s (architecture: %s)", dir, arch)

			outputFile := filepath.Join(resolvedDir, fmt.Sprintf("%s.json", dir))
			unresolvedFile := filepath.Join(unresolvedDir, fmt.Sprintf("%s.json", dir))

			// Collect all build dependencies for this repository
			buildDependencies := make([]string, 0)
			dependencySet := make(map[string]bool)
			unresolvedPackages := make([]struct {
				Package string `json:"package"`
				Error   string `json:"error"`
			}, 0)

			// Create subdirectory for detailed package info
			detailDir := filepath.Join(resolvedDir, dir)
			if err := os.MkdirAll(detailDir, 0755); err != nil {
				return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
			}

			var mu sync.Mutex

			// Process melange configurations in parallel
			var g errgroup.Group
			g.SetLimit(runtime.GOMAXPROCS(0))

			for pkgName, cfg := range pkgs {
				g.Go(func() error {
					// Capture variables for closure
					currentPkgName := pkgName
					currentCfg := cfg
					currentArch := arch

					// Check if this package supports the current architecture
					if !supportsArchitecture(currentCfg, currentArch) {
						log.Printf("Skipping %s: does not support architecture %s", currentPkgName, currentArch)
						return nil
					}

					log.Printf("Resolving build dependencies for: %s (architecture: %s)", currentPkgName, currentArch)

					// Get build repos for this directory
					repos := buildRepos[dir]

					// Use the same locking mechanism as the buckets command
					buildDeps, err := lockBuildDependencies(ctx, currentCfg, cache, repos, currentArch)
					if err != nil {
						log.Printf("Error locking build dependencies for %s (architecture: %s): %v", currentPkgName, currentArch, err)

						// Add to unresolved packages list
						mu.Lock()
						unresolvedPackages = append(unresolvedPackages, struct {
							Package string `json:"package"`
							Error   string `json:"error"`
						}{
							Package: currentPkgName,
							Error:   err.Error(),
						})
						mu.Unlock()

						return nil // Don't fail the entire operation for one config
					}

					// Save individual package dependencies to detailed JSON file
					sortedDeps := make([]string, len(buildDeps))
					copy(sortedDeps, buildDeps)
					sort.Strings(sortedDeps)

					packageDetailFile := filepath.Join(detailDir, fmt.Sprintf("%s.json", currentPkgName))
					packageData := struct {
						Package      string   `json:"package"`
						Repository   string   `json:"repository"`
						Architecture string   `json:"architecture"`
						Dependencies []string `json:"dependencies"`
					}{
						Package:      currentPkgName,
						Repository:   dir,
						Architecture: currentArch,
						Dependencies: sortedDeps,
					}

					if err := func() error {
						file, err := os.Create(packageDetailFile)
						if err != nil {
							return err
						}
						defer file.Close()

						encoder := json.NewEncoder(file)
						encoder.SetIndent("", "  ")
						return encoder.Encode(packageData)
					}(); err != nil {
						log.Printf("Warning: failed to write detailed dependencies for %s: %v", currentPkgName, err)
					}

					// Add all build dependencies to our set (with mutex protection)
					mu.Lock()
					for _, dep := range buildDeps {
						if !dependencySet[dep] {
							dependencySet[dep] = true
							buildDependencies = append(buildDependencies, dep)
						}
					}
					mu.Unlock()

					return nil
				})
			}

			if err := g.Wait(); err != nil {
				return fmt.Errorf("error processing build dependencies for %s (architecture: %s): %w", dir, arch, err)
			}

			// Sort build dependencies for consistent output
			sort.Strings(buildDependencies)

			// Create JSON structure
			data := struct {
				Repository        string   `json:"repository"`
				Architecture      string   `json:"architecture"`
				BuildDependencies []string `json:"build_dependencies"`
			}{
				Repository:        dir,
				Architecture:      arch,
				BuildDependencies: buildDependencies,
			}

			// Write build dependencies to JSON file
			file, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("creating output file %s: %w", outputFile, err)
			}
			defer file.Close()

			log.Printf("Writing %d build dependencies to %s", len(buildDependencies), outputFile)

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(data); err != nil {
				return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
			}

			log.Printf("Successfully wrote build dependencies for %s (architecture: %s) to %s", dir, arch, outputFile)

			// Write unresolved packages to JSON file if there are any
			if len(unresolvedPackages) > 0 {
				// Sort unresolved packages by package name for consistent output
				sort.Slice(unresolvedPackages, func(i, j int) bool {
					return unresolvedPackages[i].Package < unresolvedPackages[j].Package
				})

				unresolvedData := struct {
					Repository         string `json:"repository"`
					Architecture       string `json:"architecture"`
					UnresolvedPackages []struct {
						Package string `json:"package"`
						Error   string `json:"error"`
					} `json:"unresolved_packages"`
				}{
					Repository:         dir,
					Architecture:       arch,
					UnresolvedPackages: unresolvedPackages,
				}

				unresolvedFileHandle, err := os.Create(unresolvedFile)
				if err != nil {
					log.Printf("Warning: Could not create unresolved packages file %s: %v", unresolvedFile, err)
				} else {
					log.Printf("Writing %d unresolved packages to %s", len(unresolvedPackages), unresolvedFile)

					encoder := json.NewEncoder(unresolvedFileHandle)
					encoder.SetIndent("", "  ")
					if err := encoder.Encode(unresolvedData); err != nil {
						log.Printf("Warning: Error encoding unresolved packages JSON to %s: %v", unresolvedFile, err)
					} else {
						log.Printf("Successfully wrote unresolved packages for %s (architecture: %s) to %s", dir, arch, unresolvedFile)
					}
					unresolvedFileHandle.Close()
				}
			}
		}
	}

	log.Printf("Build dependency pre-computation complete for architectures %v", architectures)
	return nil
}
