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
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

func vmDependenciesCmd() *cobra.Command {
	var (
		arch string
	)

	cmd := &cobra.Command{
		Use:   "vm-dependencies",
		Short: "Pre-compute VM dependencies from wolfi-vm APKO configurations",
		Long: `This command finds all APKO build.yaml configurations under wolfi-vm/configs/**/build.yaml
and resolves them to lists of APK packages used by each VM. The results are written
to JSON files in the resolved/vms/ directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return vmDependencies(cmd.Context(), arch)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")

	return cmd
}

func vmDependencies(ctx context.Context, arch string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Pre-computing VM dependencies for architecture %s...", arch)

	// Create resolved/vms and unresolved/vms directories if they don't exist
	resolvedDir := filepath.Join("resolved", "vms")
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return fmt.Errorf("creating resolved/vms directory: %w", err)
	}

	unresolvedDir := filepath.Join("unresolved", "vms")
	if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
		return fmt.Errorf("creating unresolved/vms directory: %w", err)
	}

	cache := apk.NewCache(true)

	// Use private repos (same as private images - includes enterprise-packages)
	buildRepos := []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]}

	// Find all build.yaml files under wolfi-vm/configs/
	log.Printf("Finding VM configurations...")
	vmConfigs, err := findVMConfigs()
	if err != nil {
		return fmt.Errorf("finding VM configurations: %w", err)
	}

	log.Printf("Found %d VM configurations", len(vmConfigs))

	outputFile := filepath.Join(resolvedDir, "vms.json")
	unresolvedFile := filepath.Join(unresolvedDir, "vms.json")

	// Collect all unique APK packages across all VMs
	allPackages := make(map[string]bool) // package -> true if used
	unresolvedVMs := make([]struct {
		ConfigPath string `json:"config_path"`
		Error      string `json:"error"`
	}, 0)
	var mu sync.Mutex

	log.Printf("Resolving VM dependencies...")

	// Process VMs in parallel
	var g errgroup.Group
	g.SetLimit(runtime.GOMAXPROCS(0))

	for configPath, cfg := range vmConfigs {
		g.Go(func() error {
			// Capture variables for closure
			currentPath := configPath
			currentCfg := cfg

			log.Printf("Resolving dependencies for VM: %s", currentPath)

			// Resolve VM to APK packages
			packages, err := lockImageDependencies(ctx, currentCfg, cache, buildRepos, arch)
			if err != nil {
				log.Printf("Error resolving VM dependencies for %s: %v", currentPath, err)

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
		return fmt.Errorf("error processing VM dependencies: %w", err)
	}

	// Convert set to slice
	packageList := make([]string, 0, len(allPackages))
	for pkg := range allPackages {
		packageList = append(packageList, pkg)
	}

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

	log.Printf("Successfully wrote VM dependencies to %s", outputFile)

	// Write unresolved VMs to JSON file if there are any
	if len(unresolvedVMs) > 0 {
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
				log.Printf("Successfully wrote unresolved VMs to %s", unresolvedFile)
			}
			unresolvedFileHandle.Close()
		}
	}

	log.Printf("VM dependency pre-computation complete. Results written to %s/", resolvedDir)
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
