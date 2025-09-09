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
	AllKeptPackages  []string          `json:"all_kept_packages"` // all packages (main + subpackages) that are kept
}

func versionStreamDependenciesCmd() *cobra.Command {
	var (
		arch                   string
		useWithdrawn           bool
		withdrawnDir           string
		packageVersionMetadata string
		repositories           []string
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

			// Default to "os" repository if no repositories specified
			if len(repositories) == 0 {
				repositories = []string{"os"}
			}

			return versionStreamDependencies(cmd.Context(), architectures, useWithdrawn, withdrawnDir, packageVersionMetadata, repositories)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "", "Architecture to evaluate (default: both x86_64 and aarch64)")
	cmd.Flags().BoolVar(&useWithdrawn, "use-withdrawn", false, "Use withdrawn APKINDEX files instead of live repositories")
	cmd.Flags().StringVar(&withdrawnDir, "withdrawn-dir", "withdrawn-indexes", "Directory containing withdrawn APKINDEX files")
	cmd.Flags().StringVar(&packageVersionMetadata, "package-version-metadata", "package-version-metadata", "Path to package-version-metadata directory")
	cmd.Flags().StringSliceVar(&repositories, "repositories", []string{"os"}, "Repositories to use (os, extra-packages, enterprise-packages)")

	return cmd
}

func versionStreamDependencies(ctx context.Context, architectures []string, useWithdrawn bool, withdrawnDir, packageVersionMetadata string, repositories []string) error {
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
		var resolvedDir string
		if useWithdrawn {
			resolvedDir = filepath.Join("withdrawn-test", "resolved", "version-streams", arch)
		} else {
			resolvedDir = filepath.Join("resolved", "version-streams", arch)
		}

		if err := os.MkdirAll(resolvedDir, 0755); err != nil {
			return fmt.Errorf("creating resolved version-streams directory: %w", err)
		}

		// Note: We don't create unresolved directory for version streams -
		// not finding some versions is expected behavior

		// Create detailed output subdirectory
		detailDir := filepath.Join(resolvedDir, "packages")
		if err := os.MkdirAll(detailDir, 0755); err != nil {
			return fmt.Errorf("creating detail directory %s: %w", detailDir, err)
		}

		cache := apk.NewCache(true)

		// Build repository list from specified repositories
		var buildRepos []string
		if useWithdrawn {
			withdrawnRepos := dirToWithdrawnRepo(withdrawnDir)
			for _, repo := range repositories {
				if repoURL, ok := withdrawnRepos[repo]; ok {
					buildRepos = append(buildRepos, repoURL)
				} else {
					return fmt.Errorf("unknown repository: %s", repo)
				}
			}
		} else {
			for _, repo := range repositories {
				if repoURL, ok := dirToRepo[repo]; ok {
					buildRepos = append(buildRepos, repoURL)
				} else {
					return fmt.Errorf("unknown repository: %s", repo)
				}
			}
		}

		log.Printf("Processing version streams for architecture: %s...", arch)

		// Collect results for all version streams
		allDependencies := make(map[string]bool) // all dependencies from kept packages
		allKeptPackages := make(map[string]bool) // all newest-version packages that are kept

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
					log.Printf("Skipping version stream for %s (architecture: %s): %v", currentPackage, arch, err)
					return nil // Don't fail the entire operation for one stream - this is expected
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
				// Add all kept packages (main + subpackages)
				for _, keptPackage := range streamDeps.AllKeptPackages {
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
			ProcessedStreamsCount:     len(versionStreams),
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

	// For each stream, keep the latest version and all its subpackages
	versionStreams := make(map[string]string)
	allKeptDependencies := make(map[string]bool)
	allKeptPackages := make(map[string]bool) // Track all packages we're keeping (main + subpackages)

	for streamVersion, packages := range streamPackages {
		// Sort packages to get the latest (lexicographically last typically represents newest)
		sort.Strings(packages)
		latestPackageSpec := packages[len(packages)-1]

		// Parse the latest package spec to get name and version
		parts := strings.Split(latestPackageSpec, "=")
		if len(parts) != 2 {
			log.Printf("Warning: invalid package spec format %s, skipping", latestPackageSpec)
			continue
		}
		latestPackageName, latestPackageVersion := parts[0], parts[1]

		log.Printf("Stream %s-%s: processing latest package %s (from %d candidates)", packageName, streamVersion, latestPackageSpec, len(packages))

		versionStreams[streamVersion] = latestPackageSpec

		// Find the origin and version of the latest package to get all associated subpackages
		var originName string
		var pkgVersion string

		// Search through all indexes to find this package and get its origin
		found := false
		for _, index := range indexes {
			for _, pkg := range index.Packages {
				if pkg.Name == latestPackageName && pkg.Version == latestPackageVersion {
					originName = pkg.Origin
					pkgVersion = pkg.Version
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			log.Printf("Warning: could not find package %s in any index", latestPackageSpec)
			continue
		}

		if originName == "" {
			log.Printf("Warning: package %s has no origin, using package name as origin", latestPackageSpec)
			originName = latestPackageName
		}

		log.Printf("Stream %s-%s: found origin=%s, pkgver=%s for %s", packageName, streamVersion, originName, pkgVersion, latestPackageSpec)

		// Find ALL packages with the same origin and pkgver AND resolve their dependencies in one pass
		subpackagesFound := 0
		for _, index := range indexes {
			for _, pkg := range index.Packages {
				if pkg.Origin == originName && pkg.Version == pkgVersion {
					subpackageSpec := fmt.Sprintf("%s=%s", pkg.Name, pkg.Version)
					allKeptPackages[subpackageSpec] = true
					subpackagesFound++
					log.Printf("Stream %s-%s: keeping subpackage %s (origin=%s)", packageName, streamVersion, subpackageSpec, originName)

					// Resolve dependencies for this subpackage immediately
					log.Printf("Stream %s-%s: resolving dependencies for subpackage %s", packageName, streamVersion, subpackageSpec)

					deps, err := resolvePackageDependencies(ctx, subpackageSpec, cache, buildRepos, arch)
					if err != nil {
						log.Printf("Warning: could not resolve dependencies for %s: %v", subpackageSpec, err)
						// Continue with other subpackages even if this one fails
					} else {
						for _, dep := range deps {
							allKeptDependencies[dep] = true
						}
						log.Printf("Stream %s-%s: resolved %d dependencies for subpackage %s", packageName, streamVersion, len(deps), subpackageSpec)
					}
				}
			}
		}

		log.Printf("Stream %s-%s: found and processed %d total packages (including subpackages) for origin %s", packageName, streamVersion, subpackagesFound, originName)
	}

	// Convert dependencies set to sorted slice
	keptDependencies := make([]string, 0, len(allKeptDependencies))
	for dep := range allKeptDependencies {
		keptDependencies = append(keptDependencies, dep)
	}
	sort.Strings(keptDependencies)

	// Convert all kept packages to sorted slice
	allKeptPackagesList := make([]string, 0, len(allKeptPackages))
	for pkg := range allKeptPackages {
		allKeptPackagesList = append(allKeptPackagesList, pkg)
	}
	sort.Strings(allKeptPackagesList)

	return &VersionStreamDependencies{
		PackageName:      packageName,
		Architecture:     arch,
		VersionStreams:   versionStreams,
		KeptDependencies: keptDependencies,
		AllKeptPackages:  allKeptPackagesList, // New field for all packages (main + subpackages)
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
