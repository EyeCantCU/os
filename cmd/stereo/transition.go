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
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
)

func transitionCmd() *cobra.Command {
	var (
		packageName string
		arch        string
	)

	cmd := &cobra.Command{
		Use:   "transition",
		Short: "Manage shared library transitions for melange packages",
		Long: `This command analyzes a melange configuration with -dev package to identify
packages that need to be rebuilt during a shared library transition. It generates
regex patterns for shared libraries and determines build ordering based on dependencies.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return transition(cmd.Context(), packageName, arch)
		},
	}

	cmd.Flags().StringVar(&packageName, "package", "", "Name of the melange package driving the transition (required)")
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to evaluate (default: x86_64)")
	cmd.MarkFlagRequired("package")

	return cmd
}

func transition(ctx context.Context, packageName, arch string) error {
	// Configure log output to stderr
	log.SetOutput(os.Stderr)

	log.Printf("Analyzing shared library transition for package: %s (architecture: %s)", packageName, arch)

	// Get melange configurations
	pkgss, err := dirToPackages(ctx)
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
	packageNames := []string{targetConfig.Package.Name}
	for _, subpkg := range targetConfig.Subpackages {
		packageNames = append(packageNames, subpkg.Name)
	}

	for _, pkg := range index.Packages {
		for _, name := range packageNames {
			if pkg.Name == name {
				builtPackages = append(builtPackages, pkg)
				break
			}
		}
	}

	log.Printf("Found %d built packages from this melange config", len(builtPackages))

	// Extract shared library provides and generate regex patterns
	sharedLibPatterns, err := extractSharedLibraryPatterns(builtPackages)
	if err != nil {
		return fmt.Errorf("extracting shared library patterns: %w", err)
	}

	log.Printf("Generated %d shared library regex patterns", len(sharedLibPatterns))

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
	packagesToRebuild, err := identifyPackagesToRebuild(ctx, candidateConfigs, sharedLibPatterns, arch)
	if err != nil {
		return fmt.Errorf("identifying packages to rebuild: %w", err)
	}

	log.Printf("Identified %d packages that need rebuilding", len(packagesToRebuild))

	// Determine build order based on dependencies
	buildOrder, err := determineBuildOrder(ctx, packagesToRebuild, arch)
	if err != nil {
		return fmt.Errorf("determining build order: %w", err)
	}

	// Create output structure
	output := TransitionPlan{
		TargetPackage:         packageName,
		Repository:            targetRepo,
		Architecture:          arch,
		SharedLibraryPatterns: sharedLibPatterns,
		PackagesToRebuild:     packagesToRebuild,
		BuildOrder:            buildOrder,
	}

	// Write to JSON file
	outputFile := fmt.Sprintf("transition-%s.json", packageName)
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

type TransitionPlan struct {
	TargetPackage         string             `json:"target_package"`
	Repository            string             `json:"repository"`
	Architecture          string             `json:"architecture"`
	SharedLibraryPatterns []string           `json:"shared_library_patterns"`
	PackagesToRebuild     []RebuildCandidate `json:"packages_to_rebuild"`
	BuildOrder            [][]string         `json:"build_order"`
}

type RebuildCandidate struct {
	Name                 string   `json:"name"`
	Repository           string   `json:"repository"`
	Reason               string   `json:"reason"`
	MatchedPattern       string   `json:"matched_pattern"`
	AffectedPackages     []string `json:"affected_packages"`
}

func extractSharedLibraryPatterns(packages []*apk.Package) ([]string, error) {
	patterns := make([]string, 0)
	seenLibs := make(map[string]bool)

	for _, pkg := range packages {
		for _, provide := range pkg.Provides {
			// Look for shared library provides (format: "so:libicudata.so.75=75")
			if strings.HasPrefix(provide, "so:") {
				// Extract just the library name part (before the =)
				libPart := provide[3:] // Remove "so:" prefix
				if idx := strings.Index(libPart, "="); idx != -1 {
					libPart = libPart[:idx] // Remove version assignment
				}
				
				// Remove version numbers from the library name to create base pattern
				// Examples: libicudata.so.75 -> libicudata.so, libssl.so.3 -> libssl.so
				baseLib := removeVersionSuffix(libPart)

				if !seenLibs[baseLib] {
					// Create regex pattern that matches the base library name with any version
					// Pattern will match: so:libssl.so.1=1, so:libssl.so.3=3, etc.
					pattern := `so:` + regexp.QuoteMeta(baseLib) + `(\.\d+)*(=\d+)?`
					patterns = append(patterns, pattern)
					seenLibs[baseLib] = true
					log.Printf("Generated pattern for %s: %s", provide, pattern)
				}
			}
		}
	}

	return patterns, nil
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

func identifyPackagesToRebuild(ctx context.Context, candidates map[string]*config.Configuration, patterns []string, arch string) ([]RebuildCandidate, error) {
	log.Printf("Analyzing APK repositories for packages with shared library dependencies")

	rebuilds := make(map[string]*RebuildCandidate)

	// Compile regex patterns
	compiledPatterns := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %s: %w", pattern, err)
		}
		compiledPatterns[i] = regex
	}

	// Check all repositories for packages with matching dependencies
	for repo, repoURL := range dirToRepo {
		log.Printf("Checking APK index for repository: %s", repo)

		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repo, err)
			continue
		}

		// Check each package in the repository
		for _, pkg := range index.Packages {
			// Check if this package has dependencies matching our patterns
			needsRebuild, matchedPattern, reason := checkPackageDependencies(pkg, compiledPatterns, patterns)
			if needsRebuild {
				// Use the package's origin from the APK index to avoid duplicates
				origin := pkg.Origin
				if origin == "" {
					origin = pkg.Name // fallback to package name if no origin
				}
				
				originKey := repo + "/" + origin
				
				// Check if we have a melange config for this origin
				if _, exists := candidates[originKey]; exists {
					log.Printf("Package %s (origin: %s) needs rebuild: %s", pkg.Name, origin, reason)

					// Add or update the rebuild candidate
					if existing, exists := rebuilds[originKey]; exists {
						// Add this package to the list of affected packages
						existing.AffectedPackages = append(existing.AffectedPackages, pkg.Name)
					} else {
						// Create new rebuild candidate
						rebuilds[originKey] = &RebuildCandidate{
							Name:             origin, // Use origin name (main package)
							Repository:       repo,
							Reason:           reason,
							MatchedPattern:   matchedPattern,
							AffectedPackages: []string{pkg.Name},
						}
					}
				} else {
					log.Printf("Package %s (origin: %s) matches pattern but no melange config found", pkg.Name, origin)
				}
			}
		}
	}

	// Convert map to slice
	result := make([]RebuildCandidate, 0, len(rebuilds))
	for _, rebuild := range rebuilds {
		result = append(result, *rebuild)
	}

	return result, nil
}

func checkPackageDependencies(pkg *apk.Package, patterns []*regexp.Regexp, patternStrs []string) (bool, string, string) {
	// Check if any of the package's dependencies match our shared library patterns
	for _, dep := range pkg.Dependencies {
		for i, pattern := range patterns {
			if pattern.MatchString(dep) {
				reason := fmt.Sprintf("depends on shared library matching pattern: %s", patternStrs[i])
				return true, patternStrs[i], reason
			}
		}
	}

	return false, "", ""
}


func determineBuildOrder(ctx context.Context, packages []RebuildCandidate, arch string) ([][]string, error) {
	// Create a dependency graph based on runtime dependencies of affected packages
	
	if len(packages) == 0 {
		return [][]string{}, nil
	}

	log.Printf("Determining build order for %d packages based on runtime dependencies", len(packages))

	// Create maps for quick lookup
	packageNames := make(map[string]bool)
	packagesByName := make(map[string]*RebuildCandidate)
	
	for i := range packages {
		pkg := &packages[i]
		packageNames[pkg.Name] = true
		packagesByName[pkg.Name] = pkg
	}

	// Fetch all APK indexes to analyze runtime dependencies
	allPackages := make(map[string]*apk.Package) // packageName -> APK package info
	
	for _, repoURL := range dirToRepo {
		index, err := fetchAPKIndex(ctx, repoURL, arch)
		if err != nil {
			log.Printf("Warning: Error fetching index for dependency analysis: %v", err)
			continue
		}
		
		for _, pkg := range index.Packages {
			allPackages[pkg.Name] = pkg
		}
	}

	// Build dependency graph: package -> list of packages it depends on (within our rebuild set)
	dependencies := make(map[string][]string)
	
	// For each rebuild candidate, analyze its affected packages' runtime dependencies
	for _, pkg := range packages {
		dependencies[pkg.Name] = make([]string, 0)
		depSet := make(map[string]bool) // to avoid duplicates
		
		// Check runtime dependencies of all affected packages for this rebuild candidate
		for _, affectedPkg := range pkg.AffectedPackages {
			if apkPkg, exists := allPackages[affectedPkg]; exists {
				// Check each runtime dependency
				for _, dep := range apkPkg.Dependencies {
					// Parse dependency to get package name
					depName := dep
					if idx := strings.Index(dep, "="); idx != -1 {
						depName = dep[:idx]
					}
					if idx := strings.Index(depName, ">"); idx != -1 {
						depName = depName[:idx]
					}
					if idx := strings.Index(depName, "<"); idx != -1 {
						depName = depName[:idx]
					}
					if idx := strings.Index(depName, "~"); idx != -1 {
						depName = depName[:idx]
					}
					
					// Check if this dependency is in our rebuild set
					if packageNames[depName] && !depSet[depName] && depName != pkg.Name {
						dependencies[pkg.Name] = append(dependencies[pkg.Name], depName)
						depSet[depName] = true
						log.Printf("Found dependency: %s depends on %s", pkg.Name, depName)
					}
				}
			}
		}
	}

	// Perform topological sort
	buildOrder := make([][]string, 0)
	remaining := make(map[string]bool)
	
	for _, pkg := range packages {
		remaining[pkg.Name] = true
	}

	// Keep building levels until all packages are ordered
	for len(remaining) > 0 {
		currentLevel := make([]string, 0)

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
				currentLevel = append(currentLevel, pkgName)
			}
		}

		// If no packages can be built (circular dependency), build them all
		if len(currentLevel) == 0 {
			log.Printf("Warning: Circular dependency detected, building all remaining packages in parallel")
			for name := range remaining {
				currentLevel = append(currentLevel, name)
			}
		}

		// Remove processed packages
		for _, name := range currentLevel {
			delete(remaining, name)
		}

		buildOrder = append(buildOrder, currentLevel)
		log.Printf("Build level %d: %v", len(buildOrder), currentLevel)
	}

	return buildOrder, nil
}
