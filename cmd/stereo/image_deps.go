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
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func imageDependenciesCmd() *cobra.Command {
	var (
		private      bool
		arch         string
		useWithdrawn bool
		withdrawnDir string
	)

	cmd := &cobra.Command{
		Use:   "image-dependencies",
		Short: "Pre-compute image dependencies from terraform JSON plan",
		Long: `This command reads terraform JSON from stdin, parses all apko_build configurations,
and resolves them to lists of APK packages used by each image. The results are written
to JSON files in the resolved/images/ directory.

When no --arch is specified, dependencies are computed for both x86_64 and aarch64 architectures,
respecting any archs constraints in apko configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return imageDependencies(cmd.Context(), private, architectures, useWithdrawn, withdrawnDir)
		},
	}

	cmd.Flags().BoolVar(&private, "private", false, "Set for images-private (includes enterprise-packages)")
	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")

	return cmd
}

func imageDependencies(ctx context.Context, private bool, architectures []string, useWithdrawn bool, withdrawnDir string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Pre-computing image dependencies for architectures %v using withdrawn indexes from %s...", architectures, withdrawnDir)
	} else {
		log.Printf("Pre-computing image dependencies for architectures %v...", architectures)
	}

	var imageSet string
	if private {
		imageSet = "private"
	} else {
		imageSet = "public"
	}

	// Parse terraform JSON from stdin to get image configurations
	log.Printf("Parsing terraform plan from stdin...")
	configs, err := walk(ctx, os.Stdin)
	if err != nil {
		return fmt.Errorf("parsing terraform plan: %w", err)
	}

	log.Printf("Found %d apko_build configurations", len(configs))

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories - use withdrawn-test prefix when testing with withdrawn indexes
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "images", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "images", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "images", arch)
			unresolvedDir = filepath.Join("unresolved", "images", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved images directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved images directory: %w", err)
		}

		cache := apk.NewCache(true)

		var buildRepos map[string][]string
		if useWithdrawn {
			// Use local withdrawn indexes instead of remote repositories
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = map[string][]string{
				"public":  []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"]},
				"private": []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]},
			}
		} else {
			// Use normal remote repositories
			buildRepos = map[string][]string{
				"public":  []string{dirToRepo["os"], dirToRepo["extra-packages"]},
				"private": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
			}
		}

		outputFile := filepath.Join(resolvedDir, fmt.Sprintf("%s.json", imageSet))
		unresolvedFile := filepath.Join(unresolvedDir, fmt.Sprintf("%s.json", imageSet))

		// Collect all unique APK packages across all images
		allPackages := make(map[string]bool) // package -> true if used
		unresolvedImages := make([]struct {
			Address string `json:"address"`
			Error   string `json:"error"`
		}, 0)

		// Create subdirectory for detailed image info
		detailDir := filepath.Join(resolvedDir, imageSet)
		if err := os.MkdirAll(detailDir, 0755); err != nil {
			return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
		}

		var mu sync.Mutex

		log.Printf("Resolving image dependencies for %s images (architecture: %s)...", imageSet, arch)

		// Process images in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for addr, cfg := range configs {
			g.Go(func() error {
				// Capture variables for closure
				currentAddr := addr
				currentCfg := cfg
				currentArch := arch

				// Check if this image supports the current architecture
				if !supportsApkoArchitecture(currentCfg, currentArch) {
					log.Printf("Skipping %s: does not support architecture %s", currentAddr, currentArch)
					return nil
				}

				log.Printf("Resolving dependencies for image: %s (architecture: %s)", currentAddr, currentArch)

				// Create a copy of the configuration to avoid mutation by lockImageDependencies
				cfgCopy := *currentCfg

				// Resolve image to APK packages
				packages, err := lockImageDependencies(ctx, &cfgCopy, cache, buildRepos[imageSet], currentArch)
				if err != nil {
					log.Printf("Error resolving image dependencies for %s (architecture: %s): %v", currentAddr, currentArch, err)

					// Add to unresolved images list
					mu.Lock()
					unresolvedImages = append(unresolvedImages, struct {
						Address string `json:"address"`
						Error   string `json:"error"`
					}{
						Address: currentAddr,
						Error:   err.Error(),
					})
					mu.Unlock()

					return nil // Don't fail the entire operation for one image
				}

				// Save individual image dependencies to detailed JSON file
				sortedPkgs := make([]string, len(packages))
				copy(sortedPkgs, packages)
				sort.Strings(sortedPkgs)

				// Clean the address to create a safe filename
				safeAddr := strings.ReplaceAll(strings.ReplaceAll(currentAddr, "/", "_"), ":", "_")
				imageDetailFile := filepath.Join(detailDir, fmt.Sprintf("%s.json", safeAddr))
				imageData := struct {
					Address       string   `json:"address"`
					RepositorySet string   `json:"repository_set"`
					Architecture  string   `json:"architecture"`
					Dependencies  []string `json:"dependencies"`
				}{
					Address:       currentAddr,
					RepositorySet: imageSet,
					Architecture:  currentArch,
					Dependencies:  sortedPkgs,
				}

				if err := func() error {
					file, err := os.Create(imageDetailFile)
					if err != nil {
						return err
					}
					defer file.Close()

					encoder := json.NewEncoder(file)
					encoder.SetIndent("", "  ")
					return encoder.Encode(imageData)
				}(); err != nil {
					log.Printf("Warning: failed to write detailed dependencies for %s: %v", currentAddr, err)
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
			return fmt.Errorf("error processing image dependencies (architecture: %s): %w", arch, err)
		}

		// Convert set to sorted slice
		packageList := make([]string, 0, len(allPackages))
		for pkg := range allPackages {
			packageList = append(packageList, pkg)
		}
		sort.Strings(packageList)

		// Create JSON structure
		data := struct {
			RepositorySet     string   `json:"repository_set"`
			Architecture      string   `json:"architecture"`
			ImageDependencies []string `json:"image_dependencies"`
		}{
			RepositorySet:     imageSet,
			Architecture:      arch,
			ImageDependencies: packageList,
		}

		// Write to JSON file
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %s: %w", outputFile, err)
		}
		defer file.Close()

		log.Printf("Writing %d image dependencies to %s", len(packageList), outputFile)

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
		}

		log.Printf("Successfully wrote image dependencies for %s (architecture: %s) to %s", imageSet, arch, outputFile)

		// Write unresolved images to JSON file if there are any
		if len(unresolvedImages) > 0 {
			// Sort unresolved images by address for consistent output
			sort.Slice(unresolvedImages, func(i, j int) bool {
				return unresolvedImages[i].Address < unresolvedImages[j].Address
			})

			unresolvedData := struct {
				RepositorySet    string `json:"repository_set"`
				Architecture     string `json:"architecture"`
				UnresolvedImages []struct {
					Address string `json:"address"`
					Error   string `json:"error"`
				} `json:"unresolved_images"`
			}{
				RepositorySet:    imageSet,
				Architecture:     arch,
				UnresolvedImages: unresolvedImages,
			}

			unresolvedFileHandle, err := os.Create(unresolvedFile)
			if err != nil {
				log.Printf("Warning: Could not create unresolved images file %s: %v", unresolvedFile, err)
			} else {
				log.Printf("Writing %d unresolved images to %s", len(unresolvedImages), unresolvedFile)

				encoder := json.NewEncoder(unresolvedFileHandle)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(unresolvedData); err != nil {
					log.Printf("Warning: Error encoding unresolved images JSON to %s: %v", unresolvedFile, err)
				} else {
					log.Printf("Successfully wrote unresolved images for %s (architecture: %s) to %s", imageSet, arch, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("Image dependency pre-computation complete for architectures %v", architectures)
	return nil
}
