package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

func vmDependenciesCmd() *cobra.Command {
	var (
		arch         string
		useWithdrawn bool
		withdrawnDir string
	)

	cmd := &cobra.Command{
		Use:   "vm-dependencies",
		Short: "Pre-compute VM dependencies from wolfi-vm APKO configurations",
		Long: `This command finds all APKO build.yaml configurations under wolfi-vm/configs/**/build.yaml
and resolves them to lists of APK packages used by each VM. The results are written
to JSON files in the resolved/vms/ directory.

When no --arch is specified, dependencies are computed for both x86_64 and aarch64 architectures,
respecting any archs constraints in VM configurations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return vmDependencies(cmd.Context(), architectures, useWithdrawn, withdrawnDir)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")

	return cmd
}

func vmDependencies(ctx context.Context, architectures []string, useWithdrawn bool, withdrawnDir string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Pre-computing VM dependencies for architectures %v using withdrawn indexes from %s...", architectures, withdrawnDir)
	} else {
		log.Printf("Pre-computing VM dependencies for architectures %v...", architectures)
	}

	// Find all build.yaml files under wolfi-vm/configs/
	log.Printf("Finding VM configurations...")
	vmConfigs, err := findVMConfigs()
	if err != nil {
		return fmt.Errorf("finding VM configurations: %w", err)
	}

	log.Printf("Found %d VM configurations", len(vmConfigs))

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories - use withdrawn-test prefix when testing with withdrawn indexes
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "vms", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "vms", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "vms", arch)
			unresolvedDir = filepath.Join("unresolved", "vms", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved vms directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved vms directory: %w", err)
		}

		cache := apk.NewCache(true)

		// Build repository list for VM dependencies
		var buildRepos []string
		if useWithdrawn {
			// Use local withdrawn indexes instead of remote repositories
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]}
		} else {
			// Use private repos (same as private images - includes enterprise-packages)
			buildRepos = []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]}
		}

		outputFile := filepath.Join(resolvedDir, "vms.json")
		unresolvedFile := filepath.Join(unresolvedDir, "vms.json")

		// Collect all unique APK packages across all VMs
		allPackages := make(map[string]bool) // package -> true if used
		unresolvedVMs := make([]struct {
			ConfigPath string `json:"config_path"`
			Error      string `json:"error"`
		}, 0)

		// Create subdirectory for detailed VM info
		detailDir := filepath.Join(resolvedDir, "vms")
		if err := os.MkdirAll(detailDir, 0755); err != nil {
			return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
		}

		var mu sync.Mutex

		log.Printf("Resolving VM dependencies for architecture: %s...", arch)

		// Process VMs in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for configPath, cfg := range vmConfigs {
			g.Go(func() error {
				// Capture variables for closure
				currentPath := configPath
				currentCfg := cfg
				currentArch := arch

				// Check if this VM supports the current architecture
				if !supportsApkoArchitecture(currentCfg, currentArch) {
					log.Printf("Skipping %s: does not support architecture %s", currentPath, currentArch)
					return nil
				}

				log.Printf("Resolving dependencies for VM: %s (architecture: %s)", currentPath, currentArch)

				// Create a copy of the configuration to avoid mutation by lockImageDependencies
				cfgCopy := *currentCfg

				// Resolve VM to APK packages
				packages, err := lockImageDependencies(ctx, &cfgCopy, cache, buildRepos, currentArch)
				if err != nil {
					log.Printf("Error resolving VM dependencies for %s (architecture: %s): %v", currentPath, currentArch, err)

					// Add to unresolved VMs list
					mu.Lock()
					unresolvedVMs = append(unresolvedVMs, struct {
						ConfigPath string `json:"config_path"`
						Error      string `json:"error"`
					}{
						ConfigPath: currentPath,
						Error:      err.Error(),
					})
					mu.Unlock()

					return nil // Don't fail the entire operation for one VM
				}

				// Save individual VM dependencies to detailed JSON file
				sortedPkgs := make([]string, len(packages))
				copy(sortedPkgs, packages)
				sort.Strings(sortedPkgs)

				// Clean the path to create a safe filename
				safePath := strings.ReplaceAll(strings.ReplaceAll(currentPath, "/", "_"), ":", "_")
				vmDetailFile := filepath.Join(detailDir, fmt.Sprintf("%s.json", safePath))
				vmData := struct {
					ConfigPath   string   `json:"config_path"`
					Architecture string   `json:"architecture"`
					Dependencies []string `json:"dependencies"`
				}{
					ConfigPath:   currentPath,
					Architecture: currentArch,
					Dependencies: sortedPkgs,
				}

				if err := func() error {
					file, err := os.Create(vmDetailFile)
					if err != nil {
						return err
					}
					defer file.Close()

					encoder := json.NewEncoder(file)
					encoder.SetIndent("", "  ")
					return encoder.Encode(vmData)
				}(); err != nil {
					log.Printf("Warning: failed to write detailed dependencies for %s: %v", currentPath, err)
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
			return fmt.Errorf("error processing VM dependencies (architecture: %s): %w", arch, err)
		}

		// Convert set to sorted slice
		packageList := make([]string, 0, len(allPackages))
		for pkg := range allPackages {
			packageList = append(packageList, pkg)
		}
		sort.Strings(packageList)

		// Create JSON structure
		data := struct {
			Architecture   string   `json:"architecture"`
			VMDependencies []string `json:"vm_dependencies"`
		}{
			Architecture:   arch,
			VMDependencies: packageList,
		}

		// Write to JSON file
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %s: %w", outputFile, err)
		}
		defer file.Close()

		log.Printf("Writing %d VM dependencies to %s", len(packageList), outputFile)

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
		}

		log.Printf("Successfully wrote VM dependencies for architecture %s to %s", arch, outputFile)

		// Write unresolved VMs to JSON file if there are any
		if len(unresolvedVMs) > 0 {
			// Sort unresolved VMs by config path for consistent output
			sort.Slice(unresolvedVMs, func(i, j int) bool {
				return unresolvedVMs[i].ConfigPath < unresolvedVMs[j].ConfigPath
			})

			unresolvedData := struct {
				Architecture  string `json:"architecture"`
				UnresolvedVMs []struct {
					ConfigPath string `json:"config_path"`
					Error      string `json:"error"`
				} `json:"unresolved_vms"`
			}{
				Architecture:  arch,
				UnresolvedVMs: unresolvedVMs,
			}

			unresolvedFileHandle, err := os.Create(unresolvedFile)
			if err != nil {
				log.Printf("Warning: Could not create unresolved VMs file %s: %v", unresolvedFile, err)
			} else {
				log.Printf("Writing %d unresolved VMs to %s", len(unresolvedVMs), unresolvedFile)

				encoder := json.NewEncoder(unresolvedFileHandle)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(unresolvedData); err != nil {
					log.Printf("Warning: Error encoding unresolved VMs JSON to %s: %v", unresolvedFile, err)
				} else {
					log.Printf("Successfully wrote unresolved VMs for architecture %s to %s", arch, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("VM dependency pre-computation complete for architectures %v", architectures)
	return nil
}

func findVMConfigs() (map[string]*apko_types.ImageConfiguration, error) {
	configs := make(map[string]*apko_types.ImageConfiguration)

	// Walk the wolfi-vm/configs directory looking for build.yaml files
	err := filepath.WalkDir("wolfi-vm/configs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a regular file named build.yaml
		if !d.Type().IsRegular() || d.Name() != "build.yaml" {
			return nil
		}

		log.Printf("Found VM config: %s", path)

		// Read and parse the YAML file
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: Could not read %s: %v", path, err)
			return nil // Continue with other files
		}

		var cfg apko_types.ImageConfiguration
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("Warning: Could not parse %s: %v", path, err)
			return nil // Continue with other files
		}

		configs[path] = &cfg
		return nil
	})

	if err != nil {
		// Check if wolfi-vm directory doesn't exist
		if os.IsNotExist(err) {
			log.Printf("Warning: wolfi-vm/configs directory not found - no VM configurations to process")
			return configs, nil
		}
		return nil, fmt.Errorf("walking wolfi-vm/configs directory: %w", err)
	}

	return configs, nil
}
