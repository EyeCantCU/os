package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apko/pkg/tarfs"
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

func checkCmd() *cobra.Command {
	var (
		presubmit string
		domain    string
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run a bunch of checks and produce markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			ci, err := newCi(cmd.Context(), []string{"aarch64", "x86_64"}, false, domain, presubmit)
			if err != nil {
				return err
			}

			return ci.check(cmd.Context(), os.Stdout)
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "apk.cgr.dev", "domain for presubmit repos")
	cmd.Flags().StringVar(&presubmit, "presubmit", "", "merge sha for presubmit repos")

	return cmd
}

func impactCmd() *cobra.Command {
	var arch string
	var presubmit string
	var domain string
	var local bool
	cmd := &cobra.Command{
		Use:   "impact",
		Short: "Find melange builds that are affected by new (locally-built) packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			if presubmit == "" && !local {
				return fmt.Errorf("need to pass --local or --presubmit")
			}
			ci, err := newCi(cmd.Context(), []string{arch}, local, domain, presubmit)
			if err != nil {
				return err
			}

			result, err := ci.impact(cmd.Context())
			if err != nil {
				return err
			}

			problems := result.problems[arch]
			failures := result.failures[arch]

			for _, diff := range slices.Sorted(maps.Keys(problems)) {
				fmt.Printf("%s:\n", diff)
				for _, pkg := range problems[diff] {
					fmt.Printf("  %s\n", pkg)
				}
				fmt.Println()
			}

			// TODO: Consider somehow surfacing unguarded-ness here.
			if count := len(failures); count != 0 {
				return fmt.Errorf("%d builds have new failures", count)
			}

			log.Printf("%d melange builds affected by new packages", result.affected)

			return nil
		},
	}

	cmd.Flags().StringVar(&arch, "arch", types.ParseArchitecture(runtime.GOARCH).ToAPK(), "architecture to evaluate")
	cmd.Flags().StringVar(&domain, "domain", "apk.cgr.dev", "domain for presubmit repos")
	cmd.Flags().StringVar(&presubmit, "presubmit", "", "merge sha for presubmit repos")
	cmd.Flags().BoolVar(&local, "local", false, "whether to consider local packages")

	return cmd
}

func unguardedCmd() *cobra.Command {
	var arch string
	var ignoreFile string
	var domain string
	var presubmit string
	var local bool
	cmd := &cobra.Command{
		Use:   "unguarded",
		Short: "Find melange builds that depend on unguarded packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ignored, err := ignoredMap(ignoreFile)
			if err != nil {
				return err
			}

			ci, err := newCi(cmd.Context(), []string{arch}, local, domain, presubmit)
			if err != nil {
				return err
			}

			result, err := ci.unguarded(cmd.Context(), ignored)
			if err != nil {
				return err
			}

			problems := result.problems[arch]
			failures := result.failures[arch]

			for _, origin := range slices.Sorted(maps.Keys(problems)) {
				fmt.Printf("%s:\n", origin)
				for _, dep := range problems[origin] {
					fmt.Printf("  %s\n", dep)
				}
			}

			if len(result.notIgnored) != 0 {
				fmt.Printf("Ignored but not unguarded:\n")
				for _, pkg := range result.notIgnored {
					fmt.Printf("  %s\n", pkg)
				}
			}

			if len(failures) != 0 {
				fmt.Printf("Failed to lock %d builds:\n", len(failures))
				for _, pkg := range slices.Sorted(maps.Keys(failures)) {
					fmt.Printf("  %s\n", pkg)
				}
			}

			var errs []error
			if count := len(problems); count != 0 {
				errs = append(errs, fmt.Errorf("%d unguarded packages are still used", count))
			}
			if count := len(failures); count != 0 {
				errs = append(errs, fmt.Errorf("%d packages failed to lock", count))
			}

			return errors.Join(errs...)
		},
	}

	// Default to x86_64 because they tend to be a superset of aarch64.
	// TODO: Consider both.
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "architecture to evaluate")
	cmd.Flags().StringVar(&ignoreFile, "ignore", "unguarded.txt", "packages to ignore (we expect them to be unguarded)")

	cmd.Flags().StringVar(&domain, "domain", "apk.cgr.dev", "domain for presubmit repos")
	cmd.Flags().StringVar(&presubmit, "presubmit", "", "merge sha for presubmit repos")
	cmd.Flags().BoolVar(&local, "local", false, "whether to consider local packages")

	return cmd
}

func outdatedCmd() *cobra.Command {
	var arch string
	var ignoreFile string
	var domain string
	var presubmit string
	var local bool
	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Find melange builds that depend on old versions of packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ignored, err := ignoredMap(ignoreFile)
			if err != nil {
				return err
			}

			ci, err := newCi(cmd.Context(), []string{arch}, local, domain, presubmit)
			if err != nil {
				return err
			}

			result, err := ci.outdated(cmd.Context(), ignored)
			if err != nil {
				return err
			}

			problems := result.problems[arch]
			failures := result.failures[arch]

			for _, key := range slices.Sorted(maps.Keys(problems)) {
				fmt.Printf("%s:\n", key)
				for _, dep := range problems[key] {
					fmt.Printf("  %s\n", dep)
				}
			}

			if len(result.notIgnored) != 0 {
				fmt.Printf("Ignored but not outdated:\n")
				for _, pkg := range result.notIgnored {
					fmt.Printf("  %s\n", pkg)
				}
			}

			if len(failures) != 0 {
				fmt.Printf("Failed to lock %d builds:\n", len(failures))
				for _, pkg := range slices.Sorted(maps.Keys(failures)) {
					fmt.Printf("  %s\n", pkg)
				}
			}

			var errs []error
			if count := len(problems); count != 0 {
				errs = append(errs, fmt.Errorf("%d outdated packages in use", count))
			}
			if count := len(failures); count != 0 {
				errs = append(errs, fmt.Errorf("%d packages failed to lock", count))
			}

			return errors.Join(errs...)
		},
	}

	// Default to x86_64 because they tend to be a superset of aarch64.
	// TODO: Consider both.
	cmd.Flags().StringVar(&arch, "arch", "x86_64", "architecture to evaluate")
	cmd.Flags().StringVar(&ignoreFile, "ignore", "outdated.txt", "packages to ignore (we expect them to be outdated)")

	cmd.Flags().StringVar(&domain, "domain", "apk.cgr.dev", "domain for presubmit repos")
	cmd.Flags().StringVar(&presubmit, "presubmit", "", "merge sha for presubmit repos")
	cmd.Flags().BoolVar(&local, "local", false, "whether to consider local packages")

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
		apks := map[string]string{}

		for pkg, cfg := range pkgs {
			want := path.Join(repo, cfg.Package.Name)
			if got, ok := seen[pkg]; ok {
				errs = append(errs, fmt.Errorf("conflict: %q in %s.yaml and %s.yaml", pkg, got, want))
			}
			seen[pkg] = want
			apks[fmt.Sprintf("%s-%s.apk", pkg, cfg.Package.FullVersion())] = want
		}

		// Check that withdrawn packages are not still defined.
		withdrawnFile := filepath.Join(repo, "withdrawn-packages.txt")
		if _, err := os.Stat(withdrawnFile); os.IsNotExist(err) {
			continue
		}

		withdrawn, err := loadWithdrawnPackages(withdrawnFile)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for entry := range withdrawn {
			if file, ok := apks[entry]; ok {
				errs = append(errs, fmt.Errorf("%s/withdrawn-packages.txt contains %q still still defined in %s.yaml", repo, entry, file))
			}
		}
	}

	return errors.Join(errs...)
}

func (ci *ci) impact(ctx context.Context) (*checkResult, error) {
	problems := map[string]map[string][]string{}
	failures := map[string]map[string]error{}
	count := 0

	for arch, afterMap := range ci.depMaps {
		before := newRepoDeps("apk.cgr.dev", "", false, arch)

		beforeMap, err := lockAllBuildDeps(ctx, ci.pkgss, arch, before, ci.cache)
		if err != nil {
			return nil, err
		}

		affected := map[string]map[string]string{}

		for origin, oldDeps := range beforeMap.deps {
			newDeps, ok := afterMap.deps[origin]
			if !ok {
				return nil, fmt.Errorf("missing origin in after set: %q", origin)
			}

			if diff := diffPackages(oldDeps, newDeps); len(diff) != 0 {
				affected[origin] = diff
			}
		}

		failed := map[string]error{}
		for origin, err := range afterMap.errors {
			if _, ok := beforeMap.errors[origin]; !ok {
				failed[origin] = err
			}
		}

		inverted := map[string][]string{}
		for origin, diffs := range affected {
			for pkg, diff := range diffs {
				key := pkg + " " + diff
				inverted[key] = append(inverted[key], origin)
			}
		}

		for k := range inverted {
			slices.Sort(inverted[k])
		}

		problems[arch] = inverted
		failures[arch] = failed
		count += len(affected)
		count += len(failed)
	}

	var errs []error

	if count := maxLen(problems); count != 0 {
		errs = append(errs, fmt.Errorf("%d unguarded packages are still used", count))
	}
	if count := maxLen(failures); count != 0 {
		errs = append(errs, fmt.Errorf("%d packages failed to lock", count))
	}

	return &checkResult{
		problems: problems,
		failures: failures,
		affected: count,
	}, nil
}

func maxLen[V any](m map[string]map[string]V) int {
	tmp := 0

	for _, v := range m {
		tmp = max(tmp, len(v))
	}

	return tmp
}

type ci struct {
	archs     []string
	presubmit string
	domain    string

	pkgss map[string]map[string]*config.Configuration
	cache *apk.Cache

	// Top-level is per-arch.
	repoDeps map[string]map[string][]string
	depMaps  map[string]*depMap
}

func newCi(ctx context.Context, archs []string, local bool, domain, presubmit string) (*ci, error) {
	pkgss, err := dirToOrigins(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting package configurations: %w", err)
	}

	if domain != "apk.cgr.dev" {
		// For staging, we don't care about enterprise-packages.
		// TODO: Plumb this explicitly.
		delete(pkgss, "enterprise-packages")
	}

	cache := apk.NewCache(true)

	repoDepss := map[string]map[string][]string{}
	depMaps := map[string]*depMap{}

	for _, arch := range archs {
		repoDeps := newRepoDeps(domain, presubmit, local, arch)

		depMap, err := lockAllBuildDeps(ctx, pkgss, arch, repoDeps, cache)
		if err != nil {
			return nil, err
		}

		repoDepss[arch] = repoDeps
		depMaps[arch] = depMap
	}

	return &ci{
		archs:     archs,
		presubmit: presubmit,
		domain:    domain,
		pkgss:     pkgss,
		cache:     cache,
		repoDeps:  repoDepss,
		depMaps:   depMaps,
	}, nil
}

func (ci *ci) check(ctx context.Context, w io.Writer) error {
	ignoreUnguarded, err := ignoredMap("unguarded.txt")
	if err != nil {
		return err
	}

	var errs []error

	imp, err := ci.impact(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	if err := ci.renderImpact(w, imp); err != nil {
		errs = append(errs, err)
	}

	ung, err := ci.unguarded(ctx, ignoreUnguarded)
	if err != nil {
		errs = append(errs, err)
	}

	if err := ci.renderProblems(w, ung, "unguarded"); err != nil {
		errs = append(errs, err)
	}

	ignoreOutdated, err := ignoredMap("outdated.txt")
	if err != nil {
		return err
	}

	out, err := ci.outdated(ctx, ignoreOutdated)
	if err != nil {
		errs = append(errs, err)
	}

	if err := ci.renderProblems(w, out, "outdated"); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (ci *ci) renderImpact(w io.Writer, cr *checkResult) error {
	fmt.Fprintf(w, "## impact\n\n")
	fmt.Fprintf(w, "```\nstereo impact --presubmit %s\n```\n", ci.presubmit)

	var errs []error

	// TODO: We could probably dedupe across archs.
	for arch, problems := range cr.problems {
		failures := cr.failures[arch]

		if len(problems) == 0 && len(failures) == 0 {
			continue
		}

		fmt.Fprintf(w, "\n### %s\n\n", arch)

		for _, k := range slices.Sorted(maps.Keys(problems)) {
			fmt.Fprintf(w, "- %s\n", k)
			for _, origin := range problems[k] {
				fmt.Fprintf(w, "  - %s\n", origin)
			}
		}

		if len(failures) == 0 {
			continue
		}

		errs = append(errs, fmt.Errorf("%s: %d new lock failures", arch, len(problems)))

		fmt.Fprintf(w, "\n#### new lock failures\n\n")
		for pkg, err := range failures {
			fmt.Fprintf(w, "##### %s\n\n```\n%s\n```\n", pkg, err)
		}
	}

	return errors.Join(errs...)
}

func (ci *ci) renderProblems(w io.Writer, cr *checkResult, subcmd string) error {
	fmt.Fprintf(w, "\n## %s\n\n", subcmd)
	fmt.Fprintf(w, "```\nstereo %s --presubmit %s\n```\n", subcmd, ci.presubmit)

	var errs []error

	// TODO: We could probably dedupe across archs.
	for arch, problems := range cr.problems {
		if len(problems) == 0 {
			continue
		}

		errs = append(errs, fmt.Errorf("%s: %d %s packages", arch, len(problems), subcmd))

		fmt.Fprintf(w, "\n### %s\n\n", arch)

		fmt.Fprintf(w, "<details>\n<summary>%d packages</summary>\n\n", len(problems))

		for _, k := range slices.Sorted(maps.Keys(problems)) {
			fmt.Fprintf(w, "- %s\n", k)
			for _, origin := range problems[k] {
				fmt.Fprintf(w, "  - %s\n", origin)
			}
		}

		fmt.Fprint(w, "\n</details>\n")
	}

	if len(cr.notIgnored) != 0 {
		fmt.Fprint(w, "\n### wrongfully ignored\n\n")

		for _, line := range cr.notIgnored {
			fmt.Fprintf(w, "- %s\n", line)
		}
	}

	return errors.Join(errs...)
}

type checkResult struct {
	problems map[string]map[string][]string
	failures map[string]map[string]error

	notIgnored []string
	affected   int
}

func (ci *ci) unguarded(_ context.Context, ignored map[string]struct{}) (*checkResult, error) {
	definedPackages := map[string]struct{}{}
	for _, pkgs := range ci.pkgss {
		for name, cfg := range pkgs {
			definedPackages[name] = struct{}{}

			for _, subpkg := range cfg.Subpackages {
				definedPackages[subpkg.Name] = struct{}{}
			}
		}
	}

	// We want to see if anything in our ignored list can be dropped.
	notIgnored := maps.Clone(ignored)

	// by arch
	problems := map[string]map[string][]string{}

	for arch, depMap := range ci.depMaps {
		// pkg -> origins
		unguarded := map[string][]string{}

		for origin, deps := range depMap.deps {
			for _, dep := range deps {
				pkg, _, _ := strings.Cut(dep, "=")

				// If package exists, it's guarded.
				if _, ok := definedPackages[pkg]; ok {
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

		for k := range unguarded {
			slices.Sort(unguarded[k])
		}

		problems[arch] = unguarded
	}

	// byArch
	failures := map[string]map[string]error{}

	for arch, depMap := range ci.depMaps {
		failures[arch] = depMap.errors
	}

	var errs []error
	if count := maxLen(problems); count != 0 {
		errs = append(errs, fmt.Errorf("%d unguarded packages are still used", count))
	}
	if count := maxLen(failures); count != 0 {
		errs = append(errs, fmt.Errorf("%d packages failed to lock", count))
	}

	return &checkResult{
		problems:   problems,
		notIgnored: slices.Sorted(maps.Keys(notIgnored)),
		failures:   failures,
	}, nil
}

func (ci *ci) pushedPackages(ctx context.Context, arch string) (map[string]struct{}, error) {
	pushedPackages := map[string]struct{}{}

	// This is the superset of everything, so we'll get a list of all packages.
	superset := "enterprise-packages"
	if ci.domain != "apk.cgr.dev" {
		// For staging, we don't care about enterprise-packages.
		// TODO: Plumb this explicitly.
		superset = "extra-packages"
	}

	repoDeps := ci.repoDeps[arch]
	ic := types.ImageConfiguration{
		Contents: types.ImageContents{
			Repositories: repoDeps[superset],
		},
	}
	bc, err := build.New(ctx, tarfs.New(),
		build.WithCache("", false, ci.cache),
		build.WithImageConfiguration(ic),
		build.WithArch(types.ParseArchitecture(arch)),
	)
	if err != nil {
		return nil, err
	}

	indexes, err := bc.APK().GetRepositoryIndexes(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, idx := range indexes {
		for _, pkg := range idx.Packages() {
			pushedPackages[pkg.Name+"-"+pkg.Version] = struct{}{}
		}
	}

	return pushedPackages, nil
}

func (ci *ci) outdated(ctx context.Context, ignored map[string]struct{}) (*checkResult, error) {
	// We want to see if anything in our ignored list can be dropped.
	notIgnored := maps.Clone(ignored)

	definedPackages := map[string]string{}
	for _, pkgs := range ci.pkgss {
		for name, cfg := range pkgs {
			definedPackages[name] = cfg.Package.FullVersion()

			for _, subpkg := range cfg.Subpackages {
				definedPackages[subpkg.Name] = cfg.Package.FullVersion()
			}
		}
	}

	// by arch
	problems := map[string]map[string][]string{}

	for arch, depMap := range ci.depMaps {
		outdated := map[string][]string{}

		pushedPackages, err := ci.pushedPackages(ctx, arch)
		if err != nil {
			return nil, fmt.Errorf("fetching indexes: %w", err)
		}

		for origin, deps := range depMap.deps {
			for _, dep := range deps {
				pkg, depver, _ := strings.Cut(dep, "=")

				if pkgver, ok := definedPackages[pkg]; ok {
					pver, err := apk.ParseVersion(pkgver)
					if err != nil {
						return nil, err
					}

					dver, err := apk.ParseVersion(depver)
					if err != nil {
						return nil, err
					}

					if apk.CompareVersions(dver, pver) < 0 {
						key := fmt.Sprintf("%s-%s < %s", pkg, depver, pkgver)

						if _, ok := pushedPackages[pkg+"-"+pkgver]; !ok {
							// This means we haven't built that package yet, ignore it.
							continue
						}

						ignoreKey := fmt.Sprintf("%s-%s", pkg, depver)
						if _, ok := ignored[ignoreKey]; ok {
							delete(notIgnored, ignoreKey)
							continue
						}

						outdated[key] = append(outdated[key], origin)
					}
				}
			}
		}

		for k := range outdated {
			slices.Sort(outdated[k])
		}

		problems[arch] = outdated
	}

	// byArch
	failures := map[string]map[string]error{}

	for arch, depMap := range ci.depMaps {
		failures[arch] = depMap.errors
	}

	return &checkResult{
		problems:   problems,
		notIgnored: slices.Sorted(maps.Keys(notIgnored)),
		failures:   failures,
	}, nil
}

type depMap struct {
	errors  map[string]error
	deps    map[string][]string
	skipped map[string]struct{}
}

// TODO: build-dependencies command should probably use this
func lockAllBuildDeps(ctx context.Context, pkgss map[string]map[string]*config.Configuration, arch string, buildRepos map[string][]string, cache *apk.Cache) (*depMap, error) {
	// Create APK cache for build dependency resolution
	var mu sync.Mutex

	results := &depMap{
		errors:  map[string]error{},
		deps:    map[string][]string{},
		skipped: map[string]struct{}{},
	}

	// Process each repository directory
	for dir, pkgs := range pkgss {
		// Process melange configurations in parallel
		var g errgroup.Group
		g.SetLimit(runtime.GOMAXPROCS(0))

		for pkgName, cfg := range pkgs {
			g.Go(func() error {
				if !supportsArchitecture(cfg, arch) {
					mu.Lock()
					defer mu.Unlock()

					results.skipped[pkgName] = struct{}{}

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

var depGraph = map[string][]string{
	"os":                  {"os"},
	"extra-packages":      {"os", "extra-packages"},
	"enterprise-packages": {"os", "extra-packages", "enterprise-packages"},
}

var presubmitOrgs = map[string]string{
	"os":                  "wolfi-presubmit",
	"extra-packages":      "extra-presubmit",
	"enterprise-packages": "enterprise-presubmit",
}

func newRepoDeps(domain, presubmit string, local bool, arch string) map[string][]string {
	// prod
	repos := []map[string]string{dirToRepo}

	// presubmit
	if presubmit != "" {
		overlay := map[string]string{}

		for dir, org := range presubmitOrgs {
			overlay[dir] = fmt.Sprintf("https://%s/%s/%s", domain, org, presubmit)
		}

		repos = append(repos, overlay)
	}

	// local
	if local {
		overlay := map[string]string{}

		for _, sub := range precedence {
			if _, err := os.Stat(filepath.Join(sub, "packages", arch, "APKINDEX.tar.gz")); err == nil {
				overlay[sub] = filepath.Join(sub, "packages")
			}
		}

		repos = append(repos, overlay)
	}

	// TODO: It would be really great if we had some way to simulate withdrawing at this point.

	// Merge prod, presubmit, and local.
	repoDeps := map[string][]string{}
	for _, m := range repos {
		for repo, deps := range depGraph {
			for _, dep := range deps {
				if add, ok := m[dep]; ok {
					repoDeps[repo] = append(repoDeps[repo], add)
				}
			}
		}
	}

	if domain != "apk.cgr.dev" {
		// For staging, we don't care about enterprise-packages.
		// TODO: Plumb this explicitly.
		delete(repoDeps, "enterprise-packages")
	}

	return repoDeps
}

func diffPackages(a, b []string) map[string]string {
	before := map[string]string{}
	after := map[string]string{}

	diff := map[string]string{}

	for _, line := range a {
		pkg, ver, _ := strings.Cut(line, "=")
		before[pkg] = ver
	}

	for _, line := range b {
		pkg, ver, _ := strings.Cut(line, "=")
		after[pkg] = ver
	}

	for pkg, ver := range before {
		got, ok := after[pkg]
		if !ok {
			diff[pkg] = "- " + ver
			continue
		}

		if got != ver {
			diff[pkg] = "~ " + ver + " -> " + got
		}
	}

	for pkg, ver := range after {
		if _, ok := before[pkg]; !ok {
			diff[pkg] = "+ " + ver
		}
	}

	return diff
}

func ignoredMap(ignoreFile string) (map[string]struct{}, error) {
	ignored := map[string]struct{}{}

	f, err := os.ReadFile(ignoreFile)
	if err != nil {
		return nil, fmt.Errorf("reading ignore file %s: %w", ignoreFile, err)
	}
	for line := range bytes.Lines(f) {
		ignored[strings.TrimSuffix(strings.TrimSpace(string(line)), ".apk")] = struct{}{}
	}

	return ignored, nil
}
