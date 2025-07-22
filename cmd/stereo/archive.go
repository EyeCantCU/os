package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chainguard.dev/apko/pkg/apk/apk"
	"chainguard.dev/apko/pkg/apk/auth"
	apko_build "chainguard.dev/apko/pkg/build"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
)

func archiveCmd() *cobra.Command {
	var (
		duration  time.Duration
		outputFmt string
		arch      string
	)

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Identify APK packages that can be archived based on age and dependency criteria",
		Long: `This command identifies APK packages that can be withdrawn from APK repositories
based on the following criteria:
- Age: Packages older than the specified duration (default: 1 year)
- No reverse dependencies across any archive
- Not the most recent version if still built from origin melange configuration
- Not a reverse build dependency for any current melange configurations
- Not still in use in images, VMs, or other seeds`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return archive(cmd.Context(), duration, outputFmt, arch)
		},
	}

	cmd.Flags().DurationVar(&duration, "duration", 365*24*time.Hour, "Age threshold for archive candidates (default: 1 year)")
	cmd.Flags().StringVar(&outputFmt, "output", "text", "Output format: text, json, yaml")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")

	return cmd
}

type ArchiveCandidate struct {
	Name       string
	Version    string
	Repository string
	Age        time.Duration
	Reasons    []string
}

type ArchiveContext struct {
	DependencyMap   map[string][]Dependency          // package -> list of dependencies on it
	AllPackages     map[string][]string              // package name -> list of available versions
	PackageToOrigin map[string]string                // package name -> melange origin
	ActivePackages  map[string]*config.Configuration // packages still being built from melange
	Cache           *apk.Cache                       // APK cache for build dependency resolution
	BuildRepos      map[string][]string              // dir -> list of repo URLs for build dependencies
	ConfigToDir     map[*config.Configuration]string // config -> directory mapping
	Architecture    string                           // target architecture
}

func archive(ctx context.Context, duration time.Duration, outputFmt, arch string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Searching for APK archive candidates older than %v for architecture %s...", duration, arch)

	// Step 1: Identify older APKs
	candidates, err := findOlderAPKs(ctx, duration, arch)
	if err != nil {
		return fmt.Errorf("finding older APKs: %w", err)
	}

	log.Printf("Found %d packages older than %v", len(candidates), duration)

	// Step 2: Build archive context (fetch indexes, build dependency maps, etc.)
	archiveCtx, err := buildArchiveContext(ctx, candidates, arch)
	if err != nil {
		return fmt.Errorf("building archive context: %w", err)
	}

	// Step 3: Filter out packages with reverse dependencies
	filtered, err := filterByReverseDependencies(candidates, archiveCtx)
	if err != nil {
		return fmt.Errorf("filtering by reverse dependencies: %w", err)
	}

	log.Printf("After filtering reverse dependencies: %d packages remain", len(filtered))
	candidates = filtered

	// Step 4: Filter out most recent versions that are still built from melange
	filtered, err = filterByMostRecentVersion(candidates, archiveCtx)
	if err != nil {
		return fmt.Errorf("filtering by most recent version: %w", err)
	}

	log.Printf("After filtering most recent versions: %d packages remain", len(filtered))
	candidates = filtered

	// Step 5: Filter out packages that are reverse build dependencies
	filtered, err = filterByReverseBuildDependencies(ctx, candidates, archiveCtx)
	if err != nil {
		return fmt.Errorf("filtering by reverse build dependencies: %w", err)
	}

	log.Printf("After filtering reverse build dependencies: %d packages remain", len(filtered))
	candidates = filtered

	// Create archive directory if it doesn't exist
	archiveDir := "archive"
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}

	// Group candidates by repository for JSON output
	candidatesByRepo := make(map[string][]ArchiveCandidate)
	for _, candidate := range candidates {
		candidatesByRepo[candidate.Repository] = append(candidatesByRepo[candidate.Repository], candidate)
	}

	// Write archive candidates to JSON files per repository
	for repo, repoCandidates := range candidatesByRepo {
		archiveFile := filepath.Join(archiveDir, fmt.Sprintf("%s.json", repo))

		// Create JSON structure
		archiveData := struct {
			Repository        string `json:"repository"`
			Architecture      string `json:"architecture"`
			Duration          string `json:"duration"`
			ArchiveCandidates []struct {
				Name    string   `json:"name"`
				Version string   `json:"version"`
				Age     string   `json:"age"`
				Reasons []string `json:"reasons"`
			} `json:"archive_candidates"`
		}{
			Repository:   repo,
			Architecture: arch,
			Duration:     formatDurationInDays(duration),
			ArchiveCandidates: make([]struct {
				Name    string   `json:"name"`
				Version string   `json:"version"`
				Age     string   `json:"age"`
				Reasons []string `json:"reasons"`
			}, len(repoCandidates)),
		}

		for i, candidate := range repoCandidates {
			archiveData.ArchiveCandidates[i] = struct {
				Name    string   `json:"name"`
				Version string   `json:"version"`
				Age     string   `json:"age"`
				Reasons []string `json:"reasons"`
			}{
				Name:    candidate.Name,
				Version: candidate.Version,
				Age:     formatDurationInDays(candidate.Age),
				Reasons: candidate.Reasons,
			}
		}

		// Write to JSON file
		file, err := os.Create(archiveFile)
		if err != nil {
			log.Printf("Warning: Could not create archive file %s: %v", archiveFile, err)
		} else {
			log.Printf("Writing %d archive candidates to %s", len(repoCandidates), archiveFile)

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(archiveData); err != nil {
				log.Printf("Warning: Error encoding archive JSON to %s: %v", archiveFile, err)
			} else {
				log.Printf("Successfully wrote archive candidates for %s to %s", repo, archiveFile)
			}
			file.Close()
		}
	}

	log.Printf("Archive analysis complete. Results written to %s/", archiveDir)
	return nil
}

func findOlderAPKs(ctx context.Context, duration time.Duration, arch string) ([]ArchiveCandidate, error) {
	var candidates []ArchiveCandidate
	cutoffTime := time.Now().Add(-duration)

	// Check each repository
	for dir, repoURL := range dirToRepo {
		log.Printf("Checking repository: %s", dir)

		// Fetch the APK index for specified architecture
		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repoURL, err)
			continue
		}

		log.Printf("Processing %d packages from %s", len(index.Packages), dir)
		// Check each package in the index
		for _, pkg := range index.Packages {
			// Use the build timestamp directly
			buildTime := pkg.BuildTime

			// Check if it's older than the cutoff
			if buildTime.Before(cutoffTime) {
				candidate := ArchiveCandidate{
					Name:       pkg.Name,
					Version:    pkg.Version,
					Repository: dir,
					Age:        time.Since(buildTime),
					Reasons:    []string{"older than duration"},
				}
				candidates = append(candidates, candidate)
			}
		}
	}

	return candidates, nil
}

func fetchAPKIndex(ctx context.Context, baseURL, arch string) (*apk.APKIndex, error) {
	indexURL := fmt.Sprintf("%s/%s/APKINDEX.tar.gz", baseURL, arch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add authentication if needed
	if err := auth.DefaultAuthenticators.AddAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("adding auth: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return apk.IndexFromArchive(resp.Body)
}

func buildArchiveContext(ctx context.Context, candidates []ArchiveCandidate, arch string) (*ArchiveContext, error) {
	log.Printf("Building archive context for architecture %s...", arch)

	archiveCtx := &ArchiveContext{
		DependencyMap:   make(map[string][]Dependency),
		AllPackages:     make(map[string][]string),
		PackageToOrigin: make(map[string]string),
		ActivePackages:  make(map[string]*config.Configuration),
		Cache:           apk.NewCache(true),
		BuildRepos: map[string][]string{
			"os":                  []string{dirToRepo["os"]},
			"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
			"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
		},
		ConfigToDir:  make(map[*config.Configuration]string),
		Architecture: arch,
	}

	// Create a set of candidate packages for quick lookup
	candidateSet := make(map[string]bool)
	for _, candidate := range candidates {
		candidateSet[candidate.Name+"="+candidate.Version] = true
	}

	// Get melange configurations to understand package origins and active packages
	pkgss, err := dirToPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting package origins: %w", err)
	}

	// Map each package name to its melange origin and track active packages
	for dir, pkgs := range pkgss {
		for pkgName, cfg := range pkgs {
			archiveCtx.PackageToOrigin[pkgName] = cfg.Package.Name // The main package name is the origin
			archiveCtx.ActivePackages[pkgName] = cfg
			archiveCtx.ConfigToDir[cfg] = dir
		}
	}

	// Fetch all package indexes to build dependency graph and available packages list
	for _, repoURL := range dirToRepo {
		index, err := fetchAPKIndex(ctx, repoURL, archiveCtx.Architecture)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repoURL, err)
			continue
		}

		// For each package, track all available versions and dependencies
		for _, pkg := range index.Packages {
			// Track all available versions
			archiveCtx.AllPackages[pkg.Name] = append(archiveCtx.AllPackages[pkg.Name], pkg.Version)

			// Build dependency map
			for _, dep := range pkg.Dependencies {
				parsedDep := parseDependency(dep)
				if parsedDep.Name != "" {
					parsedDep.DependentPackage = pkg.Name
					parsedDep.DependentPackageVersion = pkg.Version
					archiveCtx.DependencyMap[parsedDep.Name] = append(archiveCtx.DependencyMap[parsedDep.Name], parsedDep)
				}
			}
		}
	}

	return archiveCtx, nil
}

func filterByReverseDependencies(candidates []ArchiveCandidate, archiveCtx *ArchiveContext) ([]ArchiveCandidate, error) {
	log.Println("Checking for reverse dependencies...")

	// Create a set of candidate packages for quick lookup
	candidateSet := make(map[string]bool)
	for _, candidate := range candidates {
		candidateSet[candidate.Name+"="+candidate.Version] = true
	}

	// Filter candidates by checking if their dependencies can be satisfied by non-candidate packages
	var filtered []ArchiveCandidate
	for _, candidate := range candidates {
		reverseDeps := archiveCtx.DependencyMap[candidate.Name]
		hasBlockingReverseDependency := false

		for _, dep := range reverseDeps {
			if versionSatisfiesDependency(candidate.Version, dep) {
				// Check if this dependency is from the same melange origin and exact version match
				candidateOrigin := archiveCtx.PackageToOrigin[candidate.Name]
				dependentOrigin := archiveCtx.PackageToOrigin[dep.DependentPackage]

				// If both packages come from the same melange origin and it's an exact version match,
				// this is an internal dependency within the same build - don't block archiving
				if candidateOrigin != "" && candidateOrigin == dependentOrigin &&
					dep.Constraint != "" && strings.HasPrefix(dep.Constraint, "=") {
					requiredVersion := dep.Constraint[1:]
					if candidate.Version == requiredVersion {
						continue
					}
				}

				// Check if this dependency can be satisfied by a non-candidate package
				canBeSatisfiedByNonCandidate := false
				availableVersions := archiveCtx.AllPackages[candidate.Name]
				for _, version := range availableVersions {
					// Skip if this version is a candidate for archiving
					if candidateSet[candidate.Name+"="+version] {
						continue
					}

					// Check if this non-candidate version satisfies the dependency
					if versionSatisfiesDependency(version, dep) {
						canBeSatisfiedByNonCandidate = true
						break
					}
				}

				if !canBeSatisfiedByNonCandidate {
					// Check if multiple candidate versions can satisfy this dependency
					satisfyingCandidates := []string{}
					for _, version := range availableVersions {
						// Only consider candidate versions
						if !candidateSet[candidate.Name+"="+version] {
							continue
						}

						// Check if this candidate version satisfies the dependency
						if versionSatisfiesDependency(version, dep) {
							satisfyingCandidates = append(satisfyingCandidates, version)
						}
					}

					if len(satisfyingCandidates) > 1 {
						// Multiple candidates can satisfy this dependency
						// Find the most recent version to keep
						mostRecentVersion := findMostRecentVersion(satisfyingCandidates)

						if candidate.Version != mostRecentVersion {
							continue // This candidate can be archived
						} else {
							hasBlockingReverseDependency = true
							break
						}
					} else {
						hasBlockingReverseDependency = true
						break
					}
				}
			}
		}

		if !hasBlockingReverseDependency {
			candidate.Reasons = append(candidate.Reasons, "no blocking reverse dependencies")
			filtered = append(filtered, candidate)
		}
	}

	return filtered, nil
}

type Dependency struct {
	Name                    string
	Constraint              string
	DependentPackage        string
	DependentPackageVersion string
}

func parseDependency(dep string) Dependency {
	// Dependencies can have version constraints like "package>=1.0.0", "package~1.0", "package=1.0.0"
	for _, op := range []string{">=", "<=", "=", "~", ">", "<"} {
		if idx := strings.Index(dep, op); idx != -1 {
			return Dependency{
				Name:       dep[:idx],
				Constraint: dep[idx:],
			}
		}
	}
	// No version constraint - any version satisfies
	return Dependency{
		Name:       dep,
		Constraint: "",
	}
}

func versionSatisfiesDependency(version string, dep Dependency) bool {
	// If no constraint (unversioned dependency), we can ignore it
	// since any version can satisfy an unversioned dependency
	if dep.Constraint == "" {
		return false
	}

	// Only consider exact version pins (=) and compatible version ranges (~)
	// as blocking dependencies
	if strings.HasPrefix(dep.Constraint, "=") {
		requiredVersion := dep.Constraint[1:]
		return version == requiredVersion
	}

	if strings.HasPrefix(dep.Constraint, "~") {
		// Tilde means compatible version - use proper APK version comparison
		requiredVersion := dep.Constraint[1:]
		return versionIsCompatible(version, requiredVersion)
	}

	// For other constraints like >=, >, <, <=, we ignore them
	// since they can typically be satisfied by newer versions
	return false
}

func versionIsCompatible(candidateVersion, requiredVersion string) bool {
	// For tilde constraints (~), use the APK module's constraint parsing
	// Create a tilde constraint and check if the candidate version satisfies it

	constraint := apk.ResolvePackageNameVersionPin("dummy~" + requiredVersion)

	candidateVer, err := apk.ParseVersion(candidateVersion)
	if err != nil {
		// If we can't parse the candidate version, assume it's not compatible
		return false
	}

	// Check if the candidate version satisfies the tilde constraint
	satisfies, err := constraint.SatisfiedBy(candidateVer)
	if err != nil {
		// If there's an error, assume it's not compatible
		return false
	}

	return satisfies
}

func findMostRecentVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	if len(versions) == 1 {
		return versions[0]
	}

	mostRecent := versions[0]
	for _, version := range versions[1:] {
		currentVer, err := apk.ParseVersion(version)
		if err != nil {
			continue
		}
		mostRecentVer, err := apk.ParseVersion(mostRecent)
		if err != nil {
			mostRecent = version
			continue
		}

		// If current version is greater than most recent, update most recent
		if apk.CompareVersions(currentVer, mostRecentVer) > 0 {
			mostRecent = version
		}
	}

	return mostRecent
}

func filterByMostRecentVersion(candidates []ArchiveCandidate, archiveCtx *ArchiveContext) ([]ArchiveCandidate, error) {
	log.Println("Checking for most recent versions still built from melange...")

	var filtered []ArchiveCandidate
	for _, candidate := range candidates {
		// Check if this package is still being built from melange
		_, isStillBuilt := archiveCtx.ActivePackages[candidate.Name]
		if !isStillBuilt {
			// Package is no longer built from melange, safe to archive any version
			candidate.Reasons = append(candidate.Reasons, "no longer built from melange")
			filtered = append(filtered, candidate)
			continue
		}

		// Package is still being built, check if this is the most recent version
		allVersions := archiveCtx.AllPackages[candidate.Name]
		if len(allVersions) <= 1 {
			// Only one version available, don't archive it
			continue
		}

		mostRecentVersion := findMostRecentVersion(allVersions)
		if candidate.Version == mostRecentVersion {
			continue
		}
		candidate.Reasons = append(candidate.Reasons, "not the most recent version")
		filtered = append(filtered, candidate)
	}

	return filtered, nil
}

func filterByReverseBuildDependencies(ctx context.Context, candidates []ArchiveCandidate, archiveCtx *ArchiveContext) ([]ArchiveCandidate, error) {
	log.Println("Checking for reverse build dependencies...")

	// Load cached build dependencies from resolved/build/ directory
	buildDependencies := make(map[string]bool) // package=version -> true if it's a build dependency

	for dir := range dirToRepo {
		buildDepsFile := filepath.Join("resolved", "build", fmt.Sprintf("%s.json", dir))

		if _, err := os.Stat(buildDepsFile); os.IsNotExist(err) {
			log.Printf("Warning: Build dependencies file not found: %s. Run 'stereo build-dependencies' first.", buildDepsFile)
			continue
		}

		file, err := os.Open(buildDepsFile)
		if err != nil {
			log.Printf("Warning: Could not open build dependencies file %s: %v", buildDepsFile, err)
			continue
		}
		defer file.Close()

		var data struct {
			Repository        string   `json:"repository"`
			Architecture      string   `json:"architecture"`
			BuildDependencies []string `json:"build_dependencies"`
		}

		if err := json.NewDecoder(file).Decode(&data); err != nil {
			log.Printf("Warning: Error decoding JSON from %s: %v", buildDepsFile, err)
			continue
		}

		for _, dep := range data.BuildDependencies {
			buildDependencies[dep] = true
		}
	}

	log.Printf("Loaded %d build dependencies from cache", len(buildDependencies))

	// Filter out candidates that are build dependencies
	var filtered []ArchiveCandidate
	for _, candidate := range candidates {
		packageVersion := candidate.Name + "=" + candidate.Version
		if buildDependencies[packageVersion] {
			continue
		}

		candidate.Reasons = append(candidate.Reasons, "not a build dependency")
		filtered = append(filtered, candidate)
	}

	return filtered, nil
}

func lockBuildDependencies(ctx context.Context, c *config.Configuration, cache *apk.Cache, apkRepos []string, arch string) ([]string, error) {
	// Work around LockImageConfiguration assuming multi-arch.
	c.Environment.Archs = []apko_types.Architecture{apko_types.Architecture(arch)}

	opts := []apko_build.Option{apko_build.WithImageConfiguration(c.Environment),
		apko_build.WithExtraBuildRepos(apkRepos),
		apko_build.WithArch(apko_types.Architecture(arch)),
		// TODO: Allow offline.
		apko_build.WithCache("", false, cache),
		// TODO: Fix that.
		apko_build.WithIgnoreSignatures(true),
	}

	configs, _, err := apko_build.LockImageConfiguration(ctx, c.Environment, opts...)
	if err != nil {
		if err := json.NewEncoder(os.Stderr).Encode(c.Environment); err != nil {
			return nil, fmt.Errorf("encoding %s: %w", c.Name)
		}
		return nil, fmt.Errorf("unable to lock image configuration: %w", err)
	}

	locked, ok := configs["index"]
	if !ok {
		return nil, errors.New("missing locked config")
	}

	return locked.Contents.Packages, nil
}

func formatDurationInDays(d time.Duration) string {
	days := d.Hours() / 24
	if days >= 1 {
		return fmt.Sprintf("%.0f days", days)
	}
	return d.String() // fallback for sub-day durations
}
