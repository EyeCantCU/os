package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func lintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Find duplicate package names",
		RunE: func(cmd *cobra.Command, args []string) error {
			return lint(cmd.Context())
		},
	}
}

func impactCmd() *cobra.Command {
	var arch string
	cmd := &cobra.Command{
		Use:   "impact",
		Short: "Find melange builds that are affected by new (locally-built) packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return impact(cmd.Context(), arch)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", types.ParseArchitecture(runtime.GOARCH).ToAPK(), "architecture to evaluate")

	return cmd
}

func unguardedCmd() *cobra.Command {
	var arch string
	var ignoreFile string
	cmd := &cobra.Command{
		Use:   "unguarded",
		Short: "Find melange builds that depend on unguarded packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ignored := map[string]struct{}{}
			if ignoreFile != "" {
				f, err := os.ReadFile(ignoreFile)
				if err != nil {
					return fmt.Errorf("reading ignore file %s: %w", err)
				}
				for line := range bytes.Lines(f) {
					ignored[strings.TrimSpace(string(line))] = struct{}{}
				}
			}
			return unguarded(cmd.Context(), arch, ignored)
		},
	}

	// Default to x86_64 because they tend to be a superset of aarch64.
	// TODO: Consider both.
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "architecture to evaluate")
	cmd.Flags().StringVar(&ignoreFile, "ignore", "unguarded.txt", "packages to ignore (we expect them to be unguarded)")

	return cmd
}

func lint(ctx context.Context) error {
	pkgss, err := dirToPackages(ctx, true)
	if err != nil {
		return err
	}

	var errs []error

	// name -> yaml path
	seen := map[string]string{}
	for repo, pkgs := range pkgss {
		for pkg, cfg := range pkgs {
			want := path.Join(repo, cfg.Package.Name)
			if got, ok := seen[pkg]; ok {
				errs = append(errs, fmt.Errorf("conflict: %q in %s.yaml and %s.yaml", pkg, got, want))
			}
			seen[pkg] = want
		}
	}

	return errors.Join(errs...)
}

func impact(ctx context.Context, arch string) error {
	// Build repository mapping for each directory
	before := map[string][]string{
		"os":                  []string{dirToRepo["os"]},
		"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
		"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	// TODO: This assumes delta is local directory.
	after := maps.Clone(before)
	for dir := range after {
		for _, sub := range []string{"os", "extra-packages", "enterprise-packages"} {
			if _, err := os.Stat(filepath.Join(sub, "packages", arch, "APKINDEX.tar.gz")); err == nil {
				after[dir] = append(after[dir], filepath.Join(sub, "packages"))
			}

			if sub == dir {
				break
			}
		}
	}

	pkgss, err := dirToOrigins(ctx)
	if err != nil {
		return fmt.Errorf("getting package configurations: %w", err)
	}

	// Share this cache across the before/after so that we don't re-fetch each prod APKINDEX.
	cache := apk.NewCache(true)

	beforeMap, err := lockAllBuildDeps(ctx, pkgss, arch, before, cache)
	if err != nil {
		return err
	}

	if errs := len(beforeMap.errors); errs != 0 {
		log.Printf("before: saw %d errors", errs)
	}

	afterMap, err := lockAllBuildDeps(ctx, pkgss, arch, after, cache)
	if err != nil {
		return err
	}

	if errs := len(afterMap.errors); errs != 0 {
		log.Printf("after: saw %d errors", errs)
	}

	affected := map[string]struct{}{}

	for origin, oldDeps := range beforeMap.deps {
		newDeps, ok := afterMap.deps[origin]
		if !ok {
			return fmt.Errorf("missing origin in after set: %q", origin)
		}

		if !slices.Equal(oldDeps, newDeps) {
			affected[origin] = struct{}{}
		}
	}

	failures := map[string]error{}

	for origin, err := range afterMap.errors {
		if _, ok := beforeMap.errors[origin]; !ok {
			failures[origin] = err
		}
	}

	for _, origin := range slices.Sorted(maps.Keys(failures)) {
		log.Printf("%s: %v", origin, failures[origin])
	}

	for _, origin := range slices.Sorted(maps.Keys(affected)) {
		newDeps := afterMap.deps[origin]
		oldDeps := beforeMap.deps[origin]

		fmt.Printf("%s:\n", origin)
		diffSorted(oldDeps, newDeps)
	}

	// TODO: Consider somehow surfacing unguarded-ness here.
	log.Printf("%d builds have new failures", len(failures))
	log.Printf("%d melange builds affected by new packages", len(affected))

	return nil
}

func unguarded(ctx context.Context, arch string, ignored map[string]struct{}) error {
	pkgss, err := dirToOrigins(ctx)
	if err != nil {
		return fmt.Errorf("getting package configurations: %w", err)
	}

	allPackages := map[string]struct{}{}
	for _, pkgs := range pkgss {
		for name, cfg := range pkgs {
			allPackages[name] = struct{}{}

			for _, subpkg := range cfg.Subpackages {
				allPackages[subpkg.Name] = struct{}{}
			}
		}
	}

	repoDeps := map[string][]string{
		"os":                  []string{dirToRepo["os"]},
		"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
		"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	depMap, err := lockAllBuildDeps(ctx, pkgss, arch, repoDeps, apk.NewCache(true))
	if err != nil {
		return err
	}

	// We want to see if anything in our ignored list can be dropped.
	notIgnored := maps.Clone(ignored)

	// pkg -> origins
	unguarded := map[string][]string{}

	for origin, deps := range depMap.deps {
		for _, dep := range deps {
			pkg, _, _ := strings.Cut(dep, "=")

			// If package exists, it's guarded.
			if _, ok := allPackages[pkg]; ok {
				continue
			}

			// If it's on the ignore list, we ignore it.
			if _, ok := ignored[pkg]; ok {
				delete(notIgnored, pkg)
				continue
			}

			unguarded[pkg] = append(unguarded[pkg], origin)
		}
	}

	for _, origin := range slices.Sorted(maps.Keys(unguarded)) {
		fmt.Printf("%s:\n", origin)
		for _, dep := range slices.Sorted(slices.Values(unguarded[origin])) {
			fmt.Printf("  %s\n", dep)
		}
	}

	if len(unguarded) != 0 {
		return fmt.Errorf("%d unguarded packages are still used", len(unguarded))
	}

	if len(notIgnored) != 0 {
		fmt.Printf("Ignored but not unguarded:\n")
		// TODO: Should this be fatal?
		for _, pkg := range slices.Sorted(maps.Keys(notIgnored)) {
			fmt.Printf("  %s\n", pkg)
		}
	}

	if len(depMap.errors) != 0 {
		// TODO: Should this be fatal?
		fmt.Printf("Failed to lock %d builds:\n", len(depMap.errors))
		for _, pkg := range slices.Sorted(maps.Keys(depMap.errors)) {
			fmt.Printf("  %s\n", pkg)
		}
	}

	return nil
}

type depMap struct {
	errors map[string]error
	deps   map[string][]string
}

// TODO: build-dependencies command should probably use this
func lockAllBuildDeps(ctx context.Context, pkgss map[string]map[string]*config.Configuration, arch string, buildRepos map[string][]string, cache *apk.Cache) (*depMap, error) {
	// Create APK cache for build dependency resolution
	var mu sync.Mutex

	results := &depMap{
		errors: map[string]error{},
		deps:   map[string][]string{},
	}

	// Process each repository directory
	for dir, pkgs := range pkgss {
		// Process melange configurations in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for pkgName, cfg := range pkgs {
			g.Go(func() error {
				if !supportsArchitecture(cfg, arch) {
					return nil
				}

				// Get build repos for this directory
				repos := buildRepos[dir]

				// Use the same locking mechanism as the buckets command
				buildDeps, err := lockBuildDependencies(ctx, cfg, cache, repos, arch)

				key := path.Join(dir, pkgName)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					results.errors[key] = err

					return nil
				}

				results.deps[key] = buildDeps

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, fmt.Errorf("error processing build dependencies for %s (architecture: %s): %w", dir, arch, err)
		}
	}

	return results, nil
}

func diffSorted(a, b []string) {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			i++
			j++
		case a[i] < b[j]:
			fmt.Printf("  - %s\n", a[i]) // only in a
			i++
		default:
			fmt.Printf("  + %s\n", b[j]) // only in b
			j++
		}
	}

	// leftover items in a
	for ; i < len(a); i++ {
		fmt.Printf("  - %s\n", a[i])
	}

	// leftover items in b
	for ; j < len(b); j++ {
		fmt.Printf("  + %s\n", b[j])
	}
}
