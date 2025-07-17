package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"chainguard.dev/apko/pkg/apk/apk"
	"chainguard.dev/apko/pkg/apk/auth"
	"github.com/spf13/cobra"
)

func archiveCmd() *cobra.Command {
	var (
		duration  time.Duration
		dryRun    bool
		outputFmt string
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
			return archive(cmd.Context(), duration, dryRun, outputFmt)
		},
	}

	cmd.Flags().DurationVar(&duration, "duration", 365*24*time.Hour, "Age threshold for archive candidates (default: 1 year)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be archived without actually doing it")
	cmd.Flags().StringVar(&outputFmt, "output", "text", "Output format: text, json, yaml")

	return cmd
}

type ArchiveCandidate struct {
	Name       string
	Version    string
	Repository string
	Age        time.Duration
	Reason     string
}

func archive(ctx context.Context, duration time.Duration, dryRun bool, outputFmt string) error {
	log.Printf("Searching for APK archive candidates older than %v...", duration)

	// Step 1: Identify older APKs
	candidates, err := findOlderAPKs(ctx, duration)
	if err != nil {
		return fmt.Errorf("finding older APKs: %w", err)
	}

	log.Printf("Found %d packages older than %v", len(candidates), duration)

	// Step 2: Filter out packages with reverse dependencies
	filtered, err := filterByReverseDependencies(ctx, candidates)
	if err != nil {
		return fmt.Errorf("filtering by reverse dependencies: %w", err)
	}

	log.Printf("After filtering reverse dependencies: %d packages remain", len(filtered))
	candidates = filtered

	if dryRun {
		log.Println("DRY RUN: Would archive the following packages:")
	} else {
		log.Println("Archive candidates:")
	}

	for _, candidate := range candidates {
		fmt.Printf("%s=%s\n", candidate.Name, candidate.Version)
	}

	return nil
}

func findOlderAPKs(ctx context.Context, duration time.Duration) ([]ArchiveCandidate, error) {
	var candidates []ArchiveCandidate
	cutoffTime := time.Now().Add(-duration)

	// Check each repository
	for dir, repoURL := range dirToRepo {
		log.Printf("Checking repository: %s (%s)", dir, repoURL)

		// Fetch the APK index for x86_64 architecture
		index, err := fetchAPKIndex(ctx, repoURL, "x86_64")
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repoURL, err)
			continue
		}

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
					Reason:     "older than duration",
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

func filterByReverseDependencies(ctx context.Context, candidates []ArchiveCandidate) ([]ArchiveCandidate, error) {
	log.Println("Checking for reverse dependencies...")

	// Build a map of all packages and their dependencies with version constraints
	dependencyMap := make(map[string][]Dependency) // package -> list of dependencies on it

	// Build a map of package name -> melange origin for determining if dependencies come from same source
	packageToOrigin := make(map[string]string)

	// Build a map of all available packages (both candidates and non-candidates)
	allPackages := make(map[string][]string) // package name -> list of available versions

	// Create a set of candidate packages for quick lookup
	candidateSet := make(map[string]bool)
	for _, candidate := range candidates {
		candidateSet[candidate.Name+"="+candidate.Version] = true
	}

	// Get melange configurations to understand package origins
	pkgss, err := dirToPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting package origins: %w", err)
	}

	// Map each package name to its melange origin
	for _, pkgs := range pkgss {
		for pkgName, cfg := range pkgs {
			packageToOrigin[pkgName] = cfg.Package.Name // The main package name is the origin
		}
	}

	// Fetch all package indexes to build dependency graph and available packages list
	for _, repoURL := range dirToRepo {
		index, err := fetchAPKIndex(ctx, repoURL, "x86_64")
		if err != nil {
			log.Printf("Error fetching index for %s: %v", repoURL, err)
			continue
		}

		// For each package, track all available versions and dependencies
		for _, pkg := range index.Packages {
			// Track all available versions
			allPackages[pkg.Name] = append(allPackages[pkg.Name], pkg.Version)

			// Build dependency map
			for _, dep := range pkg.Dependencies {
				parsedDep := parseDependency(dep)
				if parsedDep.Name != "" {
					parsedDep.DependentPackage = pkg.Name
					parsedDep.DependentPackageVersion = pkg.Version
					dependencyMap[parsedDep.Name] = append(dependencyMap[parsedDep.Name], parsedDep)
				}
			}
		}
	}

	// Filter candidates by checking if their dependencies can be satisfied by non-candidate packages
	var filtered []ArchiveCandidate
	for _, candidate := range candidates {
		reverseDeps := dependencyMap[candidate.Name]
		hasBlockingReverseDependency := false

		for _, dep := range reverseDeps {
			if versionSatisfiesDependency(candidate.Version, dep) {
				// Check if this dependency is from the same melange origin and exact version match
				candidateOrigin := packageToOrigin[candidate.Name]
				dependentOrigin := packageToOrigin[dep.DependentPackage]

				// If both packages come from the same melange origin and it's an exact version match,
				// this is an internal dependency within the same build - don't block archiving
				if candidateOrigin != "" && candidateOrigin == dependentOrigin && 
				   dep.Constraint != "" && strings.HasPrefix(dep.Constraint, "=") {
					requiredVersion := dep.Constraint[1:]
					if candidate.Version == requiredVersion {
						log.Printf("Package %s=%s internal dependency from same origin %s (%s=%s) - not blocking", 
							candidate.Name, candidate.Version, candidateOrigin, dep.DependentPackage, dep.DependentPackageVersion)
						continue
					}
				}

				// Check if this dependency can be satisfied by a non-candidate package
				canBeSatisfiedByNonCandidate := false
				availableVersions := allPackages[candidate.Name]
				for _, version := range availableVersions {
					// Skip if this version is a candidate for archiving
					if candidateSet[candidate.Name+"="+version] {
						continue
					}
					
					// Check if this non-candidate version satisfies the dependency
					if versionSatisfiesDependency(version, dep) {
						log.Printf("Package %s=%s dependency (%s) can be satisfied by non-candidate version %s=%s", 
							candidate.Name, candidate.Version, dep.Constraint, candidate.Name, version)
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
							log.Printf("Package %s=%s can be archived - dependency (%s) can be satisfied by more recent candidate %s=%s",
								candidate.Name, candidate.Version, dep.Constraint, candidate.Name, mostRecentVersion)
							continue // This candidate can be archived
						} else {
							log.Printf("Package %s=%s cannot be archived - keeping most recent of multiple candidates that satisfy dependency (%s)",
								candidate.Name, candidate.Version, dep.Constraint)
							hasBlockingReverseDependency = true
							break
						}
					} else {
						log.Printf("Package %s=%s cannot be archived - depended upon by %s (%s) and no non-candidate version can satisfy this",
							candidate.Name, candidate.Version, dep.DependentPackage, dep.Constraint)
						hasBlockingReverseDependency = true
						break
					}
				}
			}
		}

		if !hasBlockingReverseDependency {
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

