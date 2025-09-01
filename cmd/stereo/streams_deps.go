package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// VersionStream represents the structure of version stream metadata
type VersionStream struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		VersionStreamFormat string `yaml:"versionStreamFormat"`
	} `yaml:"spec"`
	Status struct {
		Versions []struct {
			Version string `yaml:"version"`
			Exists  bool   `yaml:"exists"`
		} `yaml:"versions"`
		EOLVersions []struct {
			Version string `yaml:"version"`
			Exists  bool   `yaml:"exists"`
		} `yaml:"eolVersions"`
	} `yaml:"status"`
}

// VersionStreamDependencies represents the dependencies for version-streamed packages
type VersionStreamDependencies struct {
	PackageName      string            `json:"package_name"`
	Architecture     string            `json:"architecture"`
	VersionStreams   map[string]string `json:"version_streams"`   // stream -> latest package version
	KeptDependencies []string          `json:"kept_dependencies"` // all dependencies from kept packages
}

func versionStreamDependenciesCmd() *cobra.Command {
	var (
		arch                   string
		useWithdrawn           bool
		withdrawnDir           string
		packageVersionMetadata string
	)

	cmd := &cobra.Command{
		Use:   "version-stream-dependencies",
		Short: "Process version-streamed packages in APKINDEX files",
		Long: `This command processes APKINDEX files and for each version-streamed package,
keeps only the latest version per stream while removing older versions.

For packages like postgresql-14, postgresql-15, postgresql-16, each version stream
keeps only its latest version (e.g. postgresql-14-14.19-r0) and its resolved dependencies.
Non version-streamed packages are ignored by this command.

When no --arch is specified, processes both x86_64 and aarch64 architectures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var architectures []string
			if arch != "" {
				architectures = []string{arch}
			} else {
				architectures = []string{"x86_64", "aarch64"}
			}
			return versionStreamDependencies(cmd.Context(), architectures, useWithdrawn, withdrawnDir, packageVersionMetadata)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")
	cmd.Flags().StringVar(&packageVersionMetadata, "package-version-metadata", "package-version-metadata", "Path to package-version-metadata directory")

	return cmd
}

func versionStreamDependencies(ctx context.Context, architectures []string, useWithdrawn bool, withdrawnDir, packageVersionMetadata string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	if useWithdrawn {
		log.Printf("Processing version-stream dependencies for architectures %v using withdrawn indexes from %s...", architectures, withdrawnDir)
	} else {
		log.Printf("Processing version-stream dependencies for architectures %v...", architectures)
	}

	// Load version stream configurations
	log.Printf("Loading version stream configurations from %s...", packageVersionMetadata)
	versionStreams, err := loadVersionStreams(packageVersionMetadata)
	if err != nil {
		return fmt.Errorf("loading version streams: %w", err)
	}

	if len(versionStreams) == 0 {
		log.Printf("No version streams found - nothing to process")
		return nil
	}

	log.Printf("Found %d version stream configurations", len(versionStreams))

	// Process each architecture
	for _, arch := range architectures {
		log.Printf("Processing architecture: %s", arch)

		// Create output directories
		var resolvedDir, unresolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "version-streams", arch)
			unresolvedDir = filepath.Join("withdrawn-test", "unresolved", "version-streams", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "version-streams", arch)
			unresolvedDir = filepath.Join("unresolved", "version-streams", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved version-streams directory: %w", err)
		}

		if err := os.MkdirAll(unresolvedDir, 0755); err != nil {
			return fmt.Errorf("creating unresolved version-streams directory: %w", err)
		}

		// Create detailed output subdirectory
		detailDir := filepath.Join(resolvedDir, "packages")
		if err := os.MkdirAll(detailDir, 0755); err != nil {
			return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
		}

		cache := apk.NewCache(true)

		// Build repository list
		var buildRepos []string
		if useWithdrawn {
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			buildRepos = []string{withdrawnRepos["os"], withdrawnRepos["extra-packages"], withdrawnRepos["enterprise-packages"]}
		} else {
			buildRepos = []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]}
		}

		log.Printf("Processing version streams for architecture: %s...", arch)

		// Collect results for all version streams
		allDependencies := make(map[string]bool) // all dependencies from kept packages
		allKeptPackages := make(map[string]bool) // all newest-version packages that are kept
		unresolvedStreams := make([]struct {
			PackageName string `json:"package_name"`
			Error       string `json:"error"`
		}, 0)

		var mu sync.Mutex

		// Process version streams in parallel
		var g errgroup.Group
		g.SetLimit(10) // Limit concurrent processing

		for packageName, stream := range versionStreams {
			g.Go(func() error {
				currentPackage := packageName
				currentStream := stream

				log.Printf("Processing version stream for package: %s", currentPackage)

				// Process this version stream
				streamDeps, err := processVersionStream(ctx, currentPackage, currentStream, cache, buildRepos, arch)
				if err != nil {
					log.Printf("Error processing version stream for %s (architecture: %s): %v", currentPackage, arch, err)

					mu.Lock()
					unresolvedStreams = append(unresolvedStreams, struct {
						PackageName string `json:"package_name"`
						Error       string `json:"error"`
					}{
						PackageName: currentPackage,
						Error:       err.Error(),
					})
					mu.Unlock()

					return nil // Don't fail the entire operation for one stream
				}

				// Save individual stream dependencies to detailed JSON file
				streamDetailFile := filepath.Join(detailDir, fmt.Sprintf("%s.json", currentPackage))
				if err := func() error {
					file, err := os.Create(streamDetailFile)
					if err != nil {
						return err
					}
					defer file.Close()

					encoder := json.NewEncoder(file)
					encoder.SetIndent("", "  ")
					return encoder.Encode(streamDeps)
				}(); err != nil {
					log.Printf("Warning: failed to write detailed dependencies for %s: %v", currentPackage, err)
				}

				// Add all kept dependencies and kept packages to the global sets
				mu.Lock()
				for _, dep := range streamDeps.KeptDependencies {
					allDependencies[dep] = true
				}
				// Also add the newest-version packages themselves
				for _, keptPackage := range streamDeps.VersionStreams {
					allKeptPackages[keptPackage] = true
				}
				mu.Unlock()

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("error processing version streams (architecture: %s): %w", arch, err)
		}

		// Convert kept packages to sorted slice
		keptPackagesList := make([]string, 0, len(allKeptPackages))
		for pkg := range allKeptPackages {
			keptPackagesList = append(keptPackagesList, pkg)
		}
		sort.Strings(keptPackagesList)

		// Convert dependencies to sorted slice (excluding kept packages to avoid duplication)
		dependencyList := make([]string, 0)
		for dep := range allDependencies {
			if !allKeptPackages[dep] { // Only include if it's not already in kept packages
				dependencyList = append(dependencyList, dep)
			}
		}
		sort.Strings(dependencyList)

		// Create aggregate output file
		outputFile := filepath.Join(resolvedDir, "version-streams.json")
		data := struct {
			Architecture              string   `json:"architecture"`
			KeptVersionStreamPackages []string `json:"kept_version_stream_packages"`
			Dependencies              []string `json:"dependencies"`
			ProcessedStreamsCount     int      `json:"processed_streams_count"`
			KeptPackagesCount         int      `json:"kept_packages_count"`
			DependenciesCount         int      `json:"dependencies_count"`
			TotalPackagesAndDeps      int      `json:"total_packages_and_dependencies"`
			Description               string   `json:"description"`
		}{
			Architecture:              arch,
			KeptVersionStreamPackages: keptPackagesList,
			Dependencies:              dependencyList,
			ProcessedStreamsCount:     len(versionStreams) - len(unresolvedStreams),
			KeptPackagesCount:         len(keptPackagesList),
			DependenciesCount:         len(dependencyList),
			TotalPackagesAndDeps:      len(keptPackagesList) + len(dependencyList),
			Description:               "Contains newest-version packages from version streams (kept_version_stream_packages) and their resolved dependencies (dependencies) as separate lists",
		}

		// Write to JSON file
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %s: %w", outputFile, err)
		}
		defer file.Close()

		log.Printf("Writing %d kept packages + %d dependencies from %d streams to %s", len(keptPackagesList), len(dependencyList), len(versionStreams), outputFile)

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("encoding JSON to %s: %w", outputFile, err)
		}

		log.Printf("Successfully wrote version-stream dependencies for architecture %s to %s", arch, outputFile)

		// Write unresolved streams if there are any
		if len(unresolvedStreams) > 0 {
			unresolvedFile := filepath.Join(unresolvedDir, "version-streams.json")
			unresolvedData := struct {
				Architecture      string `json:"architecture"`
				UnresolvedStreams []struct {
					PackageName string `json:"package_name"`
					Error       string `json:"error"`
				} `json:"unresolved_streams"`
			}{
				Architecture:      arch,
				UnresolvedStreams: unresolvedStreams,
			}

			unresolvedFileHandle, err := os.Create(unresolvedFile)
			if err != nil {
				log.Printf("Warning: Could not create unresolved streams file %s: %v", unresolvedFile, err)
			} else {
				log.Printf("Writing %d unresolved streams to %s", len(unresolvedStreams), unresolvedFile)

				encoder := json.NewEncoder(unresolvedFileHandle)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(unresolvedData); err != nil {
					log.Printf("Warning: Error encoding unresolved streams JSON to %s: %v", unresolvedFile, err)
				} else {
					log.Printf("Successfully wrote unresolved streams for architecture %s to %s", arch, unresolvedFile)
				}
				unresolvedFileHandle.Close()
			}
		}
	}

	log.Printf("Version-stream dependency processing complete for architectures %v", architectures)
	return nil
}

func loadVersionStreams(packageVersionMetadata string) (map[string]*VersionStream, error) {
	versionStreamsDir := filepath.Join(packageVersionMetadata, "version_streams")
	versionStreams := make(map[string]*VersionStream)

	err := filepath.WalkDir(versionStreamsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a regular YAML file
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		log.Printf("Loading version stream config: %s", path)

		// Read and parse the YAML file
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: Could not read %s: %v", path, err)
			return nil // Continue with other files
		}

		var stream VersionStream
		if err := yaml.Unmarshal(data, &stream); err != nil {
			log.Printf("Warning: Could not parse %s: %v", path, err)
			return nil // Continue with other files
		}

		// Only process version streams that have existing versions
		hasActiveVersions := false
		for _, v := range stream.Status.Versions {
			if v.Exists {
				hasActiveVersions = true
				break
			}
		}

		if !hasActiveVersions {
			log.Printf("Skipping %s: no active versions found", stream.Metadata.Name)
			return nil
		}

		versionStreams[stream.Metadata.Name] = &stream
		return nil
	})

	if err != nil {
		// Check if version_streams directory doesn't exist
		if os.IsNotExist(err) {
			log.Printf("Warning: %s directory not found - no version streams to process", versionStreamsDir)
			return versionStreams, nil
		}
		return nil, fmt.Errorf("walking %s directory: %w", versionStreamsDir, err)
	}

	return versionStreams, nil
}

func processVersionStream(ctx context.Context, packageName string, stream *VersionStream, cache *apk.Cache, buildRepos []string, arch string) (*VersionStreamDependencies, error) {
	// Download APKINDEX files from all repositories
	indexes := make([]*apk.APKIndex, 0, len(buildRepos))
	for _, repoURL := range buildRepos {
		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			return nil, fmt.Errorf("fetching APKINDEX from %s/%s: %w", repoURL, arch, err)
		}
		indexes = append(indexes, index)
	}

	// Find all packages that match the version stream pattern
	streamPackages := make(map[string][]string) // stream version -> package versions
	streamPattern, err := buildStreamPattern(packageName, stream.Spec.VersionStreamFormat)
	if err != nil {
		return nil, fmt.Errorf("building stream pattern: %w", err)
	}

	for _, index := range indexes {
		for _, pkg := range index.Packages {
			if matches := streamPattern.FindStringSubmatch(pkg.Name); matches != nil {
				streamVersion := matches[1] // The captured stream version
				packageFullName := fmt.Sprintf("%s=%s", pkg.Name, pkg.Version)
				streamPackages[streamVersion] = append(streamPackages[streamVersion], packageFullName)
			}
		}
	}

	if len(streamPackages) == 0 {
		return nil, fmt.Errorf("no packages found matching version stream pattern for %s", packageName)
	}

	log.Printf("Found %d version streams for package %s", len(streamPackages), packageName)

	// For each stream, keep only the latest version
	versionStreams := make(map[string]string)
	allKeptDependencies := make(map[string]bool)

	for streamVersion, packages := range streamPackages {
		// Sort packages to get the latest (lexicographically last typically represents newest)
		sort.Strings(packages)
		latestPackage := packages[len(packages)-1]

		log.Printf("Stream %s-%s: keeping %s (from %d candidates)", packageName, streamVersion, latestPackage, len(packages))

		versionStreams[streamVersion] = latestPackage

		// Resolve dependencies for this package
		deps, err := resolvePackageDependencies(ctx, latestPackage, cache, buildRepos, arch)
		if err != nil {
			log.Printf("Warning: could not resolve dependencies for %s: %v", latestPackage, err)
			// Continue with other streams even if this one fails
		} else {
			for _, dep := range deps {
				allKeptDependencies[dep] = true
			}
		}
	}

	// Convert dependencies set to sorted slice
	keptDependencies := make([]string, 0, len(allKeptDependencies))
	for dep := range allKeptDependencies {
		keptDependencies = append(keptDependencies, dep)
	}
	sort.Strings(keptDependencies)

	return &VersionStreamDependencies{
		PackageName:      packageName,
		Architecture:     arch,
		VersionStreams:   versionStreams,
		KeptDependencies: keptDependencies,
	}, nil
}

func buildStreamPattern(packageName, formatStr string) (*regexp.Regexp, error) {
	// Convert version stream format to regex pattern
	// Examples:
	// postgresql: ${{name}}-${{versions.major}} -> postgresql-(\d+)
	// python: ${{name}}-${{versions.major}}.${{versions.minor}} -> python-(\d+\.\d+)

	pattern := strings.ReplaceAll(formatStr, "${{name}}", regexp.QuoteMeta(packageName))
	pattern = strings.ReplaceAll(pattern, "${{versions.major}}", `(\d+)`)
	pattern = strings.ReplaceAll(pattern, "${{versions.minor}}", `\d+`)

	// If the pattern contains both major and minor, adjust to capture the full version
	if strings.Contains(formatStr, "${{versions.major}}") && strings.Contains(formatStr, "${{versions.minor}}") {
		pattern = strings.ReplaceAll(formatStr, "${{name}}", regexp.QuoteMeta(packageName))
		pattern = strings.ReplaceAll(pattern, "${{versions.major}}.${{versions.minor}}", `(\d+\.\d+)`)
	}

	// Ensure we match the exact package name
	pattern = "^" + pattern + "$"

	return regexp.Compile(pattern)
}

func resolvePackageDependencies(ctx context.Context, packageSpec string, cache *apk.Cache, buildRepos []string, arch string) ([]string, error) {
	// Create a minimal APKO configuration for this package
	cfg := &apko_types.ImageConfiguration{
		Contents: apko_types.ImageContents{
			Packages: []string{packageSpec},
		},
		Archs: []apko_types.Architecture{apko_types.Architecture(archToApkoArch(arch))},
	}

	// Use the existing lockImageDependencies function to resolve dependencies
	dependencies, err := lockImageDependencies(ctx, cfg, cache, buildRepos, arch)
	if err != nil {
		return nil, fmt.Errorf("resolving dependencies for %s: %w", packageSpec, err)
	}

	return dependencies, nil
}
