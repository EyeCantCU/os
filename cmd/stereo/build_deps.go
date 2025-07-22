package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func buildDepsCmd() *cobra.Command {
	var (
		arch string
	)

	cmd := &cobra.Command{
		Use:   "build-dependencies",
		Short: "Pre-compute build dependencies for all melange configurations",
		Long: `This command pre-computes and caches the build dependencies for all melange configurations
across the three repositories (os, extra-packages, enterprise-packages). The results are written
to files in the resolved/build/ directory and can be used by other commands to avoid expensive
dependency resolution.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildDeps(cmd.Context(), arch)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")

	return cmd
}

func buildDeps(ctx context.Context, arch string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Pre-computing build dependencies for architecture %s...", arch)

	// Create resolved/build and unresolved/build directories if they don't exist
	resolvedDir := filepath.Join("resolved", "build")
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("creating resolved/build directory: %w", err)
	}

	unresolvedDir := filepath.Join("unresolved", "build")
	if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
		return fmt.Errorf("creating unresolved/build directory: %w", err)
	}

	// Get all melange configurations
	pkgss, err := dirToPackages(ctx)
	if err != nil {
		return fmt.Errorf("getting package configurations: %w", err)
	}

	// Create APK cache for build dependency resolution
	cache := apk.NewCache(true)

	// Build repository mapping for each directory
	buildRepos := map[string][]string{
		"os":                  []string{dirToRepo["os"]},
		"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
		"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	// Process each repository directory
	for dir, pkgs := range pkgss {
		log.Printf("Processing build dependencies for repository: %s", dir)

		outputFile := filepath.Join(resolvedDir, fmt.Sprintf("%s.json", dir))
		unresolvedFile := filepath.Join(unresolvedDir, fmt.Sprintf("%s.json", dir))

		// Collect all build dependencies for this repository
		buildDependencies := make([]string, 0)
		dependencySet := make(map[string]bool)
		unresolvedPackages := make([]struct {
			Package string `json:"package"`
			Error   string `json:"error"`
		}, 0)
		var mu sync.Mutex

		// Process melange configurations in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for pkgName, cfg := range pkgs {
			g.Go(func() error {
				// Capture variables for closure
				currentPkgName := pkgName
				currentCfg := cfg

				log.Printf("Resolving build dependencies for: %s", currentPkgName)

				// Get build repos for this directory
				repos := buildRepos[dir]

				// Use the same locking mechanism as the buckets command
				buildDeps, err := lockBuildDependencies(ctx, currentCfg, cache, repos, arch)
				if err != nil {
					log.Printf("Error locking build dependencies for %s: %v", currentPkgName, err)

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
			return fmt.Errorf("error processing build dependencies for %s: %w", dir, err)
		}

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

		log.Printf("Successfully wrote build dependencies for %s to %s", dir, outputFile)

		// Write unresolved packages to JSON file if there are any
		if len(unresolvedPackages) > 0 {
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
					log.Printf("Successfully wrote unresolved packages for %s to %s", dir, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("Build dependency pre-computation complete. Results written to %s/", resolvedDir)
	return nil
}
