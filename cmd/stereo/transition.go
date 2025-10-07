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
	packagesToRebuild, completedPackages, err := identifyPackagesToRebuild(ctx, candidateConfigs, sharedLibVersions, sharedLibPatterns, arch, extraRepos)
	if err != nil {
		return fmt.Errorf("identifying packages to rebuild: %w", err)
	}

	log.Printf("Identified %d packages that need rebuilding", len(packagesToRebuild))
	log.Printf("Identified %d packages that are already complete", len(completedPackages))

	// Determine build order based on dependencies
	buildOrder, err := determineBuildOrder(ctx, packagesToRebuild, arch, extraRepos)
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
		PackagesToRebuild:     packagesToRebuild,
		CompletedPackages:     completedPackages,
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

type BuildOrderEntry struct {
	Repository string `json:"repository"`
	Package    string `json:"package"`
}

type TransitionPlan struct {
	TargetPackage         string              `json:"target_package"`
	Repository            string              `json:"repository"`
	Architecture          string              `json:"architecture"`
	SharedLibraryPatterns []string            `json:"shared_library_patterns"`
	SharedLibraryVersions map[string][]string `json:"shared_library_versions"`
	PackagesToRebuild     []RebuildCandidate  `json:"packages_to_rebuild"`
	CompletedPackages     []CompletedPackage  `json:"completed_packages"`
	BuildOrder            [][]BuildOrderEntry `json:"build_order"`
}

type AffectedPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RebuildCandidate struct {
	Name             string            `json:"name"`
	Repository       string            `json:"repository"`
	Reason           string            `json:"reason"`
	MatchedPattern   string            `json:"matched_pattern"`
	AffectedPackages []AffectedPackage `json:"affected_packages"`
}

type CompletedPackage struct {
	Name             string            `json:"name"`
	Repository       string            `json:"repository"`
	Reason           string            `json:"reason"`
	AffectedPackages []AffectedPackage `json:"affected_packages"`
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

func identifyPackagesToRebuild(ctx context.Context, candidates map[string]*config.Configuration, targetVersions map[string][]string, patterns []string, arch string, extraRepos []string) ([]RebuildCandidate, []CompletedPackage, error) {
	log.Printf("Analyzing APK repositories for packages with shared library dependencies")

	rebuilds := make(map[string]*RebuildCandidate)
	completed := make(map[string]*CompletedPackage)

	// Compile regex patterns
	compiledPatterns := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("compiling pattern %s: %w", pattern, err)
		}
		compiledPatterns[i] = regex
	}

	// First, collect all packages by origin and package name to find the most recent version of each distinct package
	originPackages := make(map[string]map[string][]*apk.Package) // originKey -> packageName -> list of package versions

	// Check dirToRepo repositories for packages
	for repo, repoURL := range dirToRepo {
		log.Printf("Checking APK index for repository: %s", repo)

		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repo, err)
			continue
		}

		// Group packages by origin and package name
		for _, pkg := range index.Packages {
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

	// For each origin, check each distinct package it builds
	for originKey, packageMap := range originPackages {
		if len(packageMap) == 0 {
			continue
		}

		repo := strings.Split(originKey, "/")[0]
		origin := strings.Split(originKey, "/")[1]

		// Track if any package from this origin needs rebuilding
		originNeedsRebuild := false
		var firstMatchedPattern, firstReason string
		affectedPackagesMap := make(map[string]*apk.Package)  // packageName -> most recent package
		completedPackagesMap := make(map[string]*apk.Package) // packageName -> most recent package
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
			needsRebuild, matchedPattern, reason := checkPackageDependenciesWithVersions(mostRecentPkg, compiledPatterns, patterns, targetVersions)
			if needsRebuild {
				log.Printf("Package %s from origin %s needs rebuild: %s", packageName, origin, reason)
				originNeedsRebuild = true
				hasMatchingDeps = true
				if firstMatchedPattern == "" {
					firstMatchedPattern = matchedPattern
					firstReason = reason
				}
				affectedPackagesMap[packageName] = mostRecentPkg
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
					Reason:           "already uses current shared library versions",
					AffectedPackages: completedAffectedPackages,
				}
			}
		}
	}

	// Convert maps to slices
	result := make([]RebuildCandidate, 0, len(rebuilds))
	for _, rebuild := range rebuilds {
		result = append(result, *rebuild)
	}

	completedResult := make([]CompletedPackage, 0, len(completed))
	for _, comp := range completed {
		completedResult = append(completedResult, *comp)
	}

	return result, completedResult, nil
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

func checkPackageDependenciesWithVersions(pkg *apk.Package, patterns []*regexp.Regexp, patternStrs []string, targetVersions map[string][]string) (bool, string, string) {
	// Check if any of the package's dependencies match our shared library patterns
	// and if they are using an older version that needs to transition to the new version
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

					// If the package depends on anything other than the newest version, it needs rebuilding
					if depVersion != newestTargetVersion {
						reason := fmt.Sprintf("depends on %s version %s, but target provides newest version %s (transition needed)", baseLib, depVersion, newestTargetVersion)
						return true, patternStrs[i], reason
					} else {
						log.Printf("Package %s already uses newest version %s of %s", pkg.Name, depVersion, baseLib)
					}
				} else if hasTarget {
					// Pattern matches but we couldn't determine versions - assume needs rebuild
					reason := fmt.Sprintf("depends on shared library matching pattern: %s (version comparison inconclusive)", patternStrs[i])
					return true, patternStrs[i], reason
				}
			}
		}
	}

	return false, "", ""
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
		// Try to parse as APK versions for proper comparison
		currentVer, err1 := apk.ParseVersion(version)
		newestVer, err2 := apk.ParseVersion(newest)

		if err1 != nil || err2 != nil {
			// If we can't parse as APK versions, do simple string comparison
			// This handles cases like "3" vs "1" where "3" > "1"
			if version > newest {
				newest = version
			}
		} else {
			// Use proper APK version comparison
			if apk.CompareVersions(currentVer, newestVer) > 0 {
				newest = version
			}
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
