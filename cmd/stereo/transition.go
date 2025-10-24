package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
)

// Reason string constants for package transition status
const (
	// ReasonVersionStreamProvider indicates a package provides older library versions
	ReasonVersionStreamProvider = "version stream provider package (maintains older library versions)"

	// ReasonAlreadyUpToDate indicates a package already uses current shared library versions
	ReasonAlreadyUpToDate = "already uses current shared library versions"

	// ReasonPinnedToVersionStream is a format string for packages pinned to older version streams
	// Arguments: baseLib, depVersion, provider
	ReasonPinnedToVersionStream = "pinned to %s version %s (provided by %s version stream)"

	// ReasonTransitionNeeded is a format string for packages needing rebuild
	// Arguments: baseLib, depVersion, newestTargetVersion
	ReasonTransitionNeeded = "depends on %s version %s, but target provides newest version %s (transition needed)"

	// ReasonVersionComparisonInconclusive is a format string when version comparison fails
	// Arguments: patternStr
	ReasonVersionComparisonInconclusive = "depends on shared library matching pattern: %s (version comparison inconclusive)"
)

func transitionCmd() *cobra.Command {
	var (
		arch       string
		extraRepos []string
	)

	cmd := &cobra.Command{
		Use:   "transition <package-name>",
		Short: "Manage shared library transitions for melange packages",
		Long: `This command analyzes a melange configuration with -dev package to identify
packages that need to be rebuilt during a shared library transition. It generates
regex patterns for shared libraries and determines build ordering based on dependencies.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return transition(cmd.Context(), args[0], arch, extraRepos)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")
	cmd.Flags().StringSliceVar(&extraRepos, "extra-repo", []string{}, "Additional APK repository URLs to include in analysis (can be specified multiple times)")

	return cmd
}

func transition(ctx context.Context, packageName, arch string, extraRepos []string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Analyzing shared library transition for package: %s (architecture: %s)", packageName, arch)
	if len(extraRepos) > 0 {
		log.Printf("Including additional repositories: %v", extraRepos)
	}

	// Get melange configurations
	pkgss, err := dirToPackages(ctx, false)
	if err != nil {
		return fmt.Errorf("getting package configurations: %w", err)
	}

	// Find the target package configuration
	var targetConfig *config.Configuration
	var targetRepo string
	for repo, pkgs := range pkgss {
		if cfg, exists := pkgs[packageName]; exists {
			targetConfig = cfg
			targetRepo = repo
			break
		}
	}

	if targetConfig == nil {
		return fmt.Errorf("package %s not found in any repository", packageName)
	}

	log.Printf("Found package %s in repository: %s", packageName, targetRepo)

	// Find the -dev package to identify what needs rebuilding
	devPackageName := packageName + "-dev"
	var devPackage *config.Subpackage

	// Check if main package is the dev package
	if strings.HasSuffix(targetConfig.Package.Name, "-dev") {
		log.Printf("Main package %s is a dev package", targetConfig.Package.Name)
		devPackageName = targetConfig.Package.Name
	} else {
		// Look for -dev subpackage
		for i := range targetConfig.Subpackages {
			if targetConfig.Subpackages[i].Name == devPackageName {
				devPackage = &targetConfig.Subpackages[i]
				break
			}
		}

		if devPackage == nil {
			return fmt.Errorf("-dev package %s not found", devPackageName)
		}
	}

	log.Printf("Found dev package: %s", devPackageName)

	// Get the current APK index to analyze provides
	repoURL := dirToRepo[targetRepo]
	log.Printf("Fetching APK index from: %s", repoURL)

	index, err := fetchAPKIndex(ctx, repoURL, arch)
	if err != nil {
		return fmt.Errorf("fetching APK index: %w", err)
	}

	// Find packages built from this melange config in the index
	builtPackages := make([]*apk.Package, 0)

	for _, pkg := range index.Packages {
		if pkg.Origin == targetConfig.Package.Name {
			builtPackages = append(builtPackages, pkg)
		}
	}

	for _, extraRepo := range extraRepos {
		log.Printf("Fetching APK index from: %s", extraRepo)

		extraIndex, err := fetchAPKIndex(ctx, extraRepo, arch)
		if err != nil {
			return fmt.Errorf("fetching APK index: %w", err)
		}
		for _, pkg := range extraIndex.Packages {
			if pkg.Origin == targetConfig.Package.Name {
				builtPackages = append(builtPackages, pkg)
			}
		}
	}

	log.Printf("Found %d built packages from this melange config", len(builtPackages))

	// Extract shared library provides and their versions
	sharedLibVersions, sharedLibPatterns, err := extractSharedLibraryPatternsWithVersions(builtPackages)
	if err != nil {
		return fmt.Errorf("extracting shared library patterns: %w", err)
	}

	log.Printf("Generated %d shared library regex patterns", len(sharedLibPatterns))
	for _, pattern := range sharedLibPatterns {
		log.Printf("Target shared library: pattern %s", pattern)
	}
	for lib, versions := range sharedLibVersions {
		log.Printf("Target shared library: %s versions %v", lib, versions)
	}

	// Find all melange configurations that might need rebuilding
	candidateConfigs := make(map[string]*config.Configuration)
	for repo, pkgs := range pkgss {
		for name, cfg := range pkgs {
			// Skip the target package itself
			if name == packageName {
				continue
			}
			candidateConfigs[repo+"/"+name] = cfg
		}
	}

	// Identify packages that need rebuilding
	analysisResult, err := identifyPackagesToRebuild(ctx, candidateConfigs, sharedLibVersions, sharedLibPatterns, arch, extraRepos, packageName)
	if err != nil {
		return fmt.Errorf("identifying packages to rebuild: %w", err)
	}

	log.Printf("Identified %d packages that need rebuilding", len(analysisResult.Rebuild))
	log.Printf("Identified %d packages that are already complete", len(analysisResult.Completed))
	log.Printf("Identified %d packages that are skipped (pinned to older version streams)", len(analysisResult.Skipped))

	// Determine build order based on dependencies
	buildOrder, err := determineBuildOrder(ctx, analysisResult.Rebuild, arch, extraRepos)
	if err != nil {
		return fmt.Errorf("determining build order: %w", err)
	}

	// Create output structure
	output := TransitionPlan{
		TargetPackage:         packageName,
		Repository:            targetRepo,
		Architecture:          arch,
		SharedLibraryPatterns: sharedLibPatterns,
		SharedLibraryVersions: sharedLibVersions,
		PackagesToRebuild:     analysisResult.Rebuild,
		CompletedPackages:     analysisResult.Completed,
		SkippedPackages:       analysisResult.Skipped,
		BuildOrder:            buildOrder,
	}

	// Create transition directory if it doesn't exist
	if err := os.MkdirAll("transition", 0755); err != nil {
		return fmt.Errorf("creating transition directory: %w", err)
	}

	// Write to JSON file
	outputFile := fmt.Sprintf("transition/%s.json", packageName)
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	log.Printf("Transition plan written to: %s", outputFile)
	fmt.Printf("Transition plan for %s written to %s\n", packageName, outputFile)

	return nil
}

// BuildOrderEntry represents a package in the build order sequence.
type BuildOrderEntry struct {
	Repository string `json:"repository"` // Repository containing the package
	Package    string `json:"package"`    // Package name to build
}

// TransitionPlan contains the complete analysis and plan for a shared library
// version transition, including which packages need rebuilding and in what order.
type TransitionPlan struct {
	TargetPackage         string              `json:"target_package"`          // Package being transitioned
	Repository            string              `json:"repository"`              // Repository of target package
	Architecture          string              `json:"architecture"`            // Target architecture
	SharedLibraryPatterns []string            `json:"shared_library_patterns"` // Regex patterns for shared libraries
	SharedLibraryVersions map[string][]string `json:"shared_library_versions"` // Library versions provided by target
	PackagesToRebuild     []RebuildCandidate  `json:"packages_to_rebuild"`     // Packages requiring rebuild
	CompletedPackages     []CompletedPackage  `json:"completed_packages"`      // Packages already up to date
	SkippedPackages       []SkippedPackage    `json:"skipped_packages"`        // Packages pinned to older versions
	BuildOrder            [][]BuildOrderEntry `json:"build_order"`             // Topologically sorted build order
}

// AffectedPackage represents a binary package affected by a transition.
type AffectedPackage struct {
	Name    string `json:"name"`    // Binary package name
	Version string `json:"version"` // Package version
}

// RebuildCandidate represents a package that needs to be rebuilt during
// a shared library transition because it depends on an outdated version.
type RebuildCandidate struct {
	Name             string            `json:"name"`              // Package origin name
	Repository       string            `json:"repository"`        // Repository where package is found
	Reason           string            `json:"reason"`            // Why rebuild is needed
	MatchedPattern   string            `json:"matched_pattern"`   // Regex pattern that matched
	AffectedPackages []AffectedPackage `json:"affected_packages"` // Binary packages that will be rebuilt
}

// CompletedPackage represents a package that has already been updated to use
// the current shared library versions and does not need rebuilding.
type CompletedPackage struct {
	Name             string            `json:"name"`              // Package origin name
	Repository       string            `json:"repository"`        // Repository where package is found
	Reason           string            `json:"reason"`            // Why it's already complete
	AffectedPackages []AffectedPackage `json:"affected_packages"` // Binary packages already up to date
}

// SkippedPackage represents a package that is intentionally skipped during a
// shared library transition because it is pinned to an older version stream
// or is itself a version stream provider package.
type SkippedPackage struct {
	Name             string            `json:"name"`              // Package origin name
	Repository       string            `json:"repository"`        // Repository where the package is found
	Reason           string            `json:"reason"`            // Human-readable reason for skipping
	PinnedVersion    string            `json:"pinned_version"`    // Version the package is pinned to (if applicable)
	ProvidedBy       string            `json:"provided_by"`       // Package providing the pinned version
	AffectedPackages []AffectedPackage `json:"affected_packages"` // List of binary packages affected
}

// PackageAnalysisResult contains the categorized results of analyzing packages
// during a shared library transition.
type PackageAnalysisResult struct {
	Rebuild   []RebuildCandidate // Packages that need to be rebuilt
	Completed []CompletedPackage // Packages already up to date
	Skipped   []SkippedPackage   // Packages pinned to older version streams
}

func extractSharedLibraryPatternsWithVersions(packages []*apk.Package) (map[string][]string, []string, error) {
	patterns := make([]string, 0)
	versions := make(map[string]map[string]bool) // base library -> set of versions provided
	seenLibs := make(map[string]bool)

	for _, pkg := range packages {
		for _, provide := range pkg.Provides {
			// Look for shared library provides (format: "so:libicudata.so.75=75")
			if strings.HasPrefix(provide, "so:") {
				// Extract the library name part
				libPart := provide[3:] // Remove "so:" prefix
				if idx := strings.Index(libPart, "="); idx != -1 {
					libPart = libPart[:idx] // Remove version assignment
				}

				// Remove version numbers from the library name to create base pattern
				// Examples: libicudata.so.75 -> libicudata.so, libssl.so.3 -> libssl.so
				baseLib := removeVersionSuffix(libPart)
				soversion := extractSoVersion(libPart)

				// Initialize the version set for this library if needed
				if versions[baseLib] == nil {
					versions[baseLib] = make(map[string]bool)
				}

				// Add this soversion to the set (automatically deduplicates)
				if soversion != "" && !versions[baseLib][soversion] {
					versions[baseLib][soversion] = true
					log.Printf("Found shared library provide: %s -> %s soversion %s", provide, baseLib, soversion)
				}

				if !seenLibs[baseLib] {
					// Create regex pattern that matches the base library name with any version
					// Pattern will match: so:libssl.so.1, so:libssl.so.3, etc.
					pattern := `so:` + regexp.QuoteMeta(baseLib) + `(\.\d+)*`
					patterns = append(patterns, pattern)
					seenLibs[baseLib] = true
				}
			}
		}
	}

	// Convert sets to slices and log summary
	distinctVersions := make(map[string][]string)
	for baseLib, versionSet := range versions {
		libVersions := make([]string, 0, len(versionSet))
		for version := range versionSet {
			libVersions = append(libVersions, version)
		}
		distinctVersions[baseLib] = libVersions
		log.Printf("Target library %s provides distinct versions: %v", baseLib, libVersions)
	}

	return distinctVersions, patterns, nil
}

func extractSoVersion(libName string) string {
	// Extract soversion from shared library names
	// Examples: libicudata.so.75 -> 75, libssl.so.3 -> 3

	// Find .so and extract anything numeric after it
	if idx := strings.Index(libName, ".so"); idx != -1 {
		remainder := libName[idx+3:] // Everything after ".so"

		// If remainder is empty, no version
		if remainder == "" {
			return ""
		}

		// Extract version suffixes like .75, .3, .1.2.3
		versionPattern := regexp.MustCompile(`^\.?(\d+(?:\.\d+)*)$`)
		matches := versionPattern.FindStringSubmatch(remainder)
		if len(matches) > 1 {
			return matches[1] // Return the captured version part
		}
	}

	return ""
}

func removeVersionSuffix(libName string) string {
	// Remove version numbers from shared library names
	// Examples: libicudata.so.75 -> libicudata.so, libssl.so.3 -> libssl.so

	// Find .so and remove anything numeric after it
	if idx := strings.Index(libName, ".so"); idx != -1 {
		base := libName[:idx+3] // Include ".so"
		remainder := libName[idx+3:]

		// If remainder is empty, return as-is
		if remainder == "" {
			return base
		}

		// Remove version suffixes like .75, .3, .1.2.3
		versionPattern := regexp.MustCompile(`^\.?\d+(\.\d+)*$`)
		if versionPattern.MatchString(remainder) {
			return base
		}
	}

	return libName
}

func identifyPackagesToRebuild(ctx context.Context, candidates map[string]*config.Configuration, targetVersions map[string][]string, patterns []string, arch string, extraRepos []string, targetPackage string) (*PackageAnalysisResult, error) {
	// Validate required parameters
	if targetPackage == "" {
		return nil, fmt.Errorf("targetPackage cannot be empty")
	}

	log.Printf("Analyzing APK repositories for packages with shared library dependencies")

	rebuilds := make(map[string]*RebuildCandidate)
	completed := make(map[string]*CompletedPackage)
	skipped := make(map[string]*SkippedPackage)

	// Build a map of which versions are actively provided by version stream packages
	// Format: baseLib -> version -> provider package name
	versionProviders := make(map[string]map[string]string)

	// Compile regex patterns
	compiledPatterns := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %s: %w", pattern, err)
		}
		compiledPatterns[i] = regex
	}

	// Scan all repositories in a single pass to:
	// 1. Identify version stream providers (packages providing older library versions)
	// 2. Collect packages by origin for dependency analysis
	log.Printf("Scanning repositories to identify version stream providers and collect packages")

	originPackages := make(map[string]map[string][]*apk.Package) // originKey -> packageName -> list of package versions

	// Process dirToRepo repositories
	for repo, repoURL := range dirToRepo {
		log.Printf("Checking APK index for repository: %s", repo)

		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repo, err)
			continue
		}

		for _, pkg := range index.Packages {
			// Check provides for shared libraries matching our patterns (for version stream detection)
			for _, provide := range pkg.Provides {
				if strings.HasPrefix(provide, "so:") {
					libPart := provide[3:]
					if idx := strings.Index(libPart, "="); idx != -1 {
						libPart = libPart[:idx]
					}

					baseLib := removeVersionSuffix(libPart)
					soversion := extractSoVersion(libPart)

					// Check if this matches one of our target libraries
					if _, isTarget := targetVersions[baseLib]; isTarget && soversion != "" {
						if versionProviders[baseLib] == nil {
							versionProviders[baseLib] = make(map[string]string)
						}
						// Record which package provides this version
						// If multiple packages provide the same version, prefer the one with a version suffix in its name
						// (e.g., prefer "protobuf-29.5" over "protobuf" for version 29.5.0)
						if existingProvider, exists := versionProviders[baseLib][soversion]; !exists {
							versionProviders[baseLib][soversion] = pkg.Origin
							log.Printf("Version stream: %s version %s provided by %s", baseLib, soversion, pkg.Origin)
						} else if existingProvider != pkg.Origin {
							log.Printf("Note: %s version %s provided by both %s and %s", baseLib, soversion, existingProvider, pkg.Origin)
							// Prefer versioned package names (e.g., protobuf-29.5 over protobuf)
							// Check if the new origin has a version suffix like -29.5 or -3.11
							versionSuffixPattern := regexp.MustCompile(`-\d+(\.\d+)*$`)
							if versionSuffixPattern.MatchString(pkg.Origin) {
								versionProviders[baseLib][soversion] = pkg.Origin
								log.Printf("Preferring versioned provider %s for %s version %s", pkg.Origin, baseLib, soversion)
							}
						}
					}
				}
			}

			// Collect packages by origin and package name
			originKey := repo + "/" + pkg.Origin

			// Check if we have a melange config for this origin
			if config, exists := candidates[originKey]; exists {
				// Verify that this APK package is still built by the melange configuration
				if isPackageBuiltByConfig(pkg.Name, config) {
					if originPackages[originKey] == nil {
						originPackages[originKey] = make(map[string][]*apk.Package)
					}
					originPackages[originKey][pkg.Name] = append(originPackages[originKey][pkg.Name], pkg)
				} else {
					log.Printf("Skipping package %s with origin %s - no longer built by melange config", pkg.Name, pkg.Origin)
				}
			}
		}
	}

	// Check extra repositories for packages, trying to match them against all known melange configs
	for _, extraRepoURL := range extraRepos {
		log.Printf("Checking APK index for extra repository: %s", extraRepoURL)

		index, err := fetchAPKIndex(ctx, extraRepoURL, arch)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", extraRepoURL, err)
			continue
		}

		// Group packages by origin and package name
		for _, pkg := range index.Packages {
			// Try to find a melange config for this origin in any of the source repositories
			for repo := range dirToRepo {
				originKey := repo + "/" + pkg.Origin
				if config, exists := candidates[originKey]; exists {
					// Verify that this APK package is still built by the melange configuration
					if isPackageBuiltByConfig(pkg.Name, config) {
						if originPackages[originKey] == nil {
							originPackages[originKey] = make(map[string][]*apk.Package)
						}
						originPackages[originKey][pkg.Name] = append(originPackages[originKey][pkg.Name], pkg)
						log.Printf("Found package %s from extra repo matching melange config in %s", pkg.Name, repo)
					}
					break // Found matching config, no need to check other repos
				}
			}
		}
	}

	// Build a set of version stream provider origins to exclude from rebuild checks
	versionStreamOrigins := make(map[string]bool)
	for _, providers := range versionProviders {
		for _, provider := range providers {
			versionStreamOrigins[provider] = true
		}
	}

	// For each origin, check each distinct package it builds
	for originKey, packageMap := range originPackages {
		if len(packageMap) == 0 {
			continue
		}

		repo := strings.Split(originKey, "/")[0]
		origin := strings.Split(originKey, "/")[1]

		// Skip version stream packages themselves - they provide the older versions
		if versionStreamOrigins[origin] {
			log.Printf("Skipping %s - it is a version stream provider package", origin)

			// Add it to skipped packages with a special reason
			affectedPackages := make([]AffectedPackage, 0, len(packageMap))
			for _, packageVersions := range packageMap {
				mostRecentPkg := packageVersions[0]
				for _, pkg := range packageVersions[1:] {
					if isMoreRecent(pkg, mostRecentPkg) {
						mostRecentPkg = pkg
					}
				}
				affectedPackages = append(affectedPackages, AffectedPackage{
					Name:    mostRecentPkg.Name,
					Version: mostRecentPkg.Version,
				})
			}

			skipped[originKey] = &SkippedPackage{
				Name:             origin,
				Repository:       repo,
				Reason:           ReasonVersionStreamProvider,
				PinnedVersion:    "",
				ProvidedBy:       origin,
				AffectedPackages: affectedPackages,
			}
			continue
		}

		// Track if any package from this origin needs rebuilding
		originNeedsRebuild := false
		originIsPinned := false
		var firstMatchedPattern, firstReason, pinnedVersion, providedBy string
		affectedPackagesMap := make(map[string]*apk.Package)  // packageName -> most recent package
		completedPackagesMap := make(map[string]*apk.Package) // packageName -> most recent package
		pinnedPackagesMap := make(map[string]*apk.Package)    // packageName -> most recent package
		hasMatchingDeps := false

		// Check each distinct package name from this origin
		for packageName, packageVersions := range packageMap {
			// Find the most recent version of this specific package
			mostRecentPkg := packageVersions[0]
			for _, pkg := range packageVersions[1:] {
				if isMoreRecent(pkg, mostRecentPkg) {
					mostRecentPkg = pkg
				}
			}

			log.Printf("Checking most recent version of %s from origin %s: %s v%s", packageName, origin, mostRecentPkg.Name, mostRecentPkg.Version)

			// Check if this package has dependencies matching our patterns
			status := checkPackageDependenciesWithVersions(mostRecentPkg, compiledPatterns, patterns, targetVersions, versionProviders, targetPackage)
			if status.NeedsRebuild {
				log.Printf("Package %s from origin %s needs rebuild: %s", packageName, origin, status.Reason)
				originNeedsRebuild = true
				hasMatchingDeps = true
				if firstMatchedPattern == "" {
					firstMatchedPattern = status.MatchedPattern
					firstReason = status.Reason
				}
				affectedPackagesMap[packageName] = mostRecentPkg
			} else if status.IsPinned {
				log.Printf("Package %s from origin %s is pinned: %s", packageName, origin, status.Reason)
				originIsPinned = true
				hasMatchingDeps = true
				if firstMatchedPattern == "" {
					firstMatchedPattern = status.MatchedPattern
					firstReason = status.Reason
					pinnedVersion = status.PinnedVersion
					providedBy = status.ProvidedBy
				}
				pinnedPackagesMap[packageName] = mostRecentPkg
			} else {
				// Check if package has any dependencies matching our patterns (even if up to date)
				hasMatch := false
				for _, dep := range mostRecentPkg.Dependencies {
					for _, pattern := range compiledPatterns {
						if pattern.MatchString(dep) {
							hasMatch = true
							hasMatchingDeps = true
							break
						}
					}
					if hasMatch {
						break
					}
				}
				if hasMatch {
					log.Printf("Package %s from origin %s already uses current shared library versions", packageName, origin)
					completedPackagesMap[packageName] = mostRecentPkg
				}
			}

		}

		// If any package from this origin needs rebuilding, create a rebuild candidate
		if originNeedsRebuild {
			log.Printf("Origin %s needs rebuild", origin)

			// Convert to AffectedPackage slice
			affectedPackages := make([]AffectedPackage, 0, len(affectedPackagesMap))
			for _, pkg := range affectedPackagesMap {
				affectedPackages = append(affectedPackages, AffectedPackage{
					Name:    pkg.Name,
					Version: pkg.Version,
				})
			}

			rebuilds[originKey] = &RebuildCandidate{
				Name:             origin, // Use origin name (main package)
				Repository:       repo,
				Reason:           firstReason,
				MatchedPattern:   firstMatchedPattern,
				AffectedPackages: affectedPackages,
			}
		} else if originIsPinned {
			log.Printf("Origin %s is pinned to older version stream", origin)

			// Convert to AffectedPackage slice for pinned packages
			pinnedAffectedPackages := make([]AffectedPackage, 0, len(pinnedPackagesMap))
			for _, pkg := range pinnedPackagesMap {
				pinnedAffectedPackages = append(pinnedAffectedPackages, AffectedPackage{
					Name:    pkg.Name,
					Version: pkg.Version,
				})
			}

			if len(pinnedAffectedPackages) > 0 {
				skipped[originKey] = &SkippedPackage{
					Name:             origin, // Use origin name (main package)
					Repository:       repo,
					Reason:           firstReason,
					PinnedVersion:    pinnedVersion,
					ProvidedBy:       providedBy,
					AffectedPackages: pinnedAffectedPackages,
				}
			}
		} else if hasMatchingDeps {
			log.Printf("Origin %s already uses current shared library versions", origin)

			// Convert to AffectedPackage slice for completed packages
			completedAffectedPackages := make([]AffectedPackage, 0, len(completedPackagesMap))
			for _, pkg := range completedPackagesMap {
				completedAffectedPackages = append(completedAffectedPackages, AffectedPackage{
					Name:    pkg.Name,
					Version: pkg.Version,
				})
			}

			if len(completedAffectedPackages) > 0 {
				completed[originKey] = &CompletedPackage{
					Name:             origin, // Use origin name (main package)
					Repository:       repo,
					Reason:           ReasonAlreadyUpToDate,
					AffectedPackages: completedAffectedPackages,
				}
			}
		}
	}

	// Convert maps to slices and return as structured result
	result := &PackageAnalysisResult{
		Rebuild:   make([]RebuildCandidate, 0, len(rebuilds)),
		Completed: make([]CompletedPackage, 0, len(completed)),
		Skipped:   make([]SkippedPackage, 0, len(skipped)),
	}

	for _, rebuild := range rebuilds {
		result.Rebuild = append(result.Rebuild, *rebuild)
	}

	for _, comp := range completed {
		result.Completed = append(result.Completed, *comp)
	}

	for _, skip := range skipped {
		result.Skipped = append(result.Skipped, *skip)
	}

	return result, nil
}

func isMoreRecent(pkg1, pkg2 *apk.Package) bool {
	// Compare package versions
	ver1, err1 := apk.ParseVersion(pkg1.Version)
	ver2, err2 := apk.ParseVersion(pkg2.Version)

	if err1 != nil || err2 != nil {
		// If we can't parse versions, prefer the one with parseable version
		return err1 == nil && err2 != nil
	}

	return apk.CompareVersions(ver1, ver2) > 0
}

// PackageTransitionStatus represents the transition state of a package
// during a shared library version migration. It indicates whether a package
// needs to be rebuilt, is pinned to an older version stream, or is already
// up to date.
type PackageTransitionStatus struct {
	NeedsRebuild   bool   // Package must be rebuilt for the new library version
	IsPinned       bool   // Package is intentionally locked to an older version stream
	MatchedPattern string // Regex pattern that matched the dependency
	Reason         string // Human-readable explanation of the status
	PinnedVersion  string // Version the package is pinned to (if IsPinned is true)
	ProvidedBy     string // Package providing the pinned version (if IsPinned is true)
}

// checkPackageDependenciesWithVersions analyzes a package's dependencies to determine
// its transition status during a shared library version migration.
//
// It checks if the package depends on shared libraries matching the target patterns and:
// - Returns NeedsRebuild=true if the package uses an older library version that should be updated
// - Returns IsPinned=true if the package is intentionally pinned to an older version stream
// - Returns NeedsRebuild=false and IsPinned=false if the package is already up to date
//
// A package is considered pinned if it depends on an older library version that is actively
// provided by a different version stream package (not the target package itself).
func checkPackageDependenciesWithVersions(pkg *apk.Package, patterns []*regexp.Regexp, patternStrs []string, targetVersions map[string][]string, versionProviders map[string]map[string]string, targetPackage string) PackageTransitionStatus {
	for _, dep := range pkg.Dependencies {
		for i, pattern := range patterns {
			if pattern.MatchString(dep) {
				// Extract the dependency soversion (no = in dependencies)
				var depVersion string
				if strings.HasPrefix(dep, "so:") {
					// For shared library dependencies like "so:libssl.so.3" (no = in dependencies)
					libPart := dep[3:] // Remove "so:" prefix
					depVersion = extractSoVersion(libPart)
				}

				// Get the base library name to check against target versions
				baseLib := extractBaseLibraryName(dep)
				targetVersionsList, hasTarget := targetVersions[baseLib]

				if hasTarget && depVersion != "" && len(targetVersionsList) > 0 {
					// Find the newest version provided by the target (this is what we're transitioning TO)
					newestTargetVersion := findNewestVersion(targetVersionsList)

					// If the package depends on anything other than the newest version
					if depVersion != newestTargetVersion {
						// Check if this older version is still actively provided by a version stream package
						// BUT: only consider it pinned if it's provided by a DIFFERENT package than the target
						if providers, hasProviders := versionProviders[baseLib]; hasProviders {
							if provider, isPinned := providers[depVersion]; isPinned && provider != targetPackage {
								// This package is pinned to an older version stream (not the target package itself)
								reason := fmt.Sprintf(ReasonPinnedToVersionStream, baseLib, depVersion, provider)
								log.Printf("Package %s is pinned: %s", pkg.Name, reason)
								return PackageTransitionStatus{
									NeedsRebuild:   false,
									IsPinned:       true,
									MatchedPattern: patternStrs[i],
									Reason:         reason,
									PinnedVersion:  depVersion,
									ProvidedBy:     provider,
								}
							}
						}

						// Not pinned, needs rebuild
						reason := fmt.Sprintf(ReasonTransitionNeeded, baseLib, depVersion, newestTargetVersion)
						return PackageTransitionStatus{
							NeedsRebuild:   true,
							IsPinned:       false,
							MatchedPattern: patternStrs[i],
							Reason:         reason,
						}
					} else {
						log.Printf("Package %s already uses newest version %s of %s", pkg.Name, depVersion, baseLib)
					}
				} else if hasTarget {
					// Pattern matches but we couldn't determine versions - assume needs rebuild
					reason := fmt.Sprintf(ReasonVersionComparisonInconclusive, patternStrs[i])
					return PackageTransitionStatus{
						NeedsRebuild:   true,
						IsPinned:       false,
						MatchedPattern: patternStrs[i],
						Reason:         reason,
					}
				}
			}
		}
	}

	return PackageTransitionStatus{
		NeedsRebuild: false,
		IsPinned:     false,
	}
}

func findNewestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	if len(versions) == 1 {
		return versions[0]
	}

	newest := versions[0]
	for _, version := range versions[1:] {
		// Parse as APK versions for proper comparison
		currentVer, err1 := apk.ParseVersion(version)
		newestVer, err2 := apk.ParseVersion(newest)

		if err1 != nil {
			log.Printf("Warning: failed to parse version %q as APK version: %v", version, err1)
			continue
		}
		if err2 != nil {
			log.Printf("Warning: failed to parse version %q as APK version: %v", newest, err2)
			// If current parses but newest doesn't, use current
			newest = version
			continue
		}

		// Use proper APK version comparison
		if apk.CompareVersions(currentVer, newestVer) > 0 {
			newest = version
		}
	}

	return newest
}

func extractBaseLibraryName(dep string) string {
	// Extract base library name from dependency like "so:libssl.so.3=3"
	if strings.HasPrefix(dep, "so:") {
		libPart := dep[3:] // Remove "so:" prefix
		if idx := strings.Index(libPart, "="); idx != -1 {
			libPart = libPart[:idx] // Remove version assignment
		}

		// Remove version numbers from the library name
		return removeVersionSuffix(libPart)
	}

	return dep
}

func determineBuildOrder(ctx context.Context, packages []RebuildCandidate, arch string, extraRepos []string) ([][]BuildOrderEntry, error) {
	// Create a dependency graph based on dependencies using lockImageConfiguration

	if len(packages) == 0 {
		return [][]BuildOrderEntry{}, nil
	}

	log.Printf("Determining build order for %d packages based on dependencies", len(packages))

	// Create maps for quick lookup
	packageNames := make(map[string]bool)
	packagesByName := make(map[string]*RebuildCandidate)

	for i := range packages {
		pkg := &packages[i]
		packageNames[pkg.Name] = true
		packagesByName[pkg.Name] = pkg
	}

	// Build dependency graph using full dependency resolution
	dependencies := make(map[string][]string)
	cache := apk.NewCache(true)

	// Create repository list including extra repos if provided
	repoURLs := make([]string, 0, len(dirToRepo)+len(extraRepos))
	for _, url := range dirToRepo {
		repoURLs = append(repoURLs, url)
	}
	for _, extraRepo := range extraRepos {
		repoURLs = append(repoURLs, extraRepo)
	}

	// For each rebuild candidate, resolve dependencies
	for _, pkg := range packages {
		dependencies[pkg.Name] = make([]string, 0)
		depSet := make(map[string]bool) // to avoid duplicates

		log.Printf("Resolving dependencies for %s", pkg.Name)

		// Create a dummy ImageConfiguration with all affected packages from this rebuild candidate
		packageList := make([]string, 0, len(pkg.AffectedPackages))
		for _, affectedPkg := range pkg.AffectedPackages {
			packageList = append(packageList, affectedPkg.Name)
		}

		// Create dummy config
		dummyConfig := &config.Configuration{
			Environment: apko_types.ImageConfiguration{
				Contents: apko_types.ImageContents{
					Packages: packageList,
				},
				Archs: []apko_types.Architecture{apko_types.Architecture(arch)},
			},
		}

		// Resolve dependencies
		fullPackages, err := lockBuildDependencies(ctx, dummyConfig, cache, repoURLs, arch)
		if err != nil {
			log.Printf("Warning: Could not resolve dependencies for %s: %v", pkg.Name, err)
			continue
		}

		log.Printf("Found %d dependencies for %s", len(fullPackages), pkg.Name)

		// Check which dependencies are in our rebuild set
		for _, fullPkg := range fullPackages {
			// Parse package name from package=version format
			pkgName := fullPkg
			if idx := strings.Index(fullPkg, "="); idx != -1 {
				pkgName = fullPkg[:idx]
			}

			// Check if this dependency is in our rebuild set and is not the package itself
			if packageNames[pkgName] && !depSet[pkgName] && pkgName != pkg.Name {
				dependencies[pkg.Name] = append(dependencies[pkg.Name], pkgName)
				depSet[pkgName] = true
				log.Printf("Found dependency: %s depends on %s", pkg.Name, pkgName)
			}
		}
	}

	// Perform topological sort
	buildOrder := make([][]BuildOrderEntry, 0)
	remaining := make(map[string]bool)

	for _, pkg := range packages {
		remaining[pkg.Name] = true
	}

	// Keep building levels until all packages are ordered
	for len(remaining) > 0 {
		currentLevelPkgs := make([]string, 0)

		// Find packages with no unresolved dependencies in remaining set
		for pkgName := range remaining {
			hasUnresolvedDeps := false

			for _, depName := range dependencies[pkgName] {
				if remaining[depName] {
					hasUnresolvedDeps = true
					break
				}
			}

			if !hasUnresolvedDeps {
				currentLevelPkgs = append(currentLevelPkgs, pkgName)
			}
		}

		// If no packages can be built (circular dependency), build them all
		if len(currentLevelPkgs) == 0 {
			log.Printf("Warning: Circular dependency detected, building all remaining packages in parallel")
			for name := range remaining {
				currentLevelPkgs = append(currentLevelPkgs, name)
			}
		}

		// Group packages by their repository directory
		repoGroups := make(map[string][]string)
		for _, pkgName := range currentLevelPkgs {
			if pkg, exists := packagesByName[pkgName]; exists {
				repoGroups[pkg.Repository] = append(repoGroups[pkg.Repository], pkgName)
			}
		}

		// Create grouped build level entries
		currentLevel := make([]BuildOrderEntry, 0)
		for repo, pkgs := range repoGroups {
			// Add all packages from this repo as structured entries
			for _, pkg := range pkgs {
				currentLevel = append(currentLevel, BuildOrderEntry{
					Repository: repo,
					Package:    pkg,
				})
			}
		}

		// Remove processed packages
		for _, name := range currentLevelPkgs {
			delete(remaining, name)
		}

		buildOrder = append(buildOrder, currentLevel)
		log.Printf("Build level %d: %v", len(buildOrder), currentLevel)
	}

	return buildOrder, nil
}

func isPackageBuiltByConfig(packageName string, cfg *config.Configuration) bool {
	// Check if the package matches the main package name
	if packageName == cfg.Package.Name {
		return true
	}

	// Check if the package matches any subpackage name
	for _, subpkg := range cfg.Subpackages {
		if packageName == subpkg.Name {
			return true
		}
	}

	return false
}
