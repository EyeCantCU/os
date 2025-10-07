package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"

	"chainguard.dev/apko/pkg/apk/apk"
	apko_build "chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/melange/pkg/config"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func bucketsCmd() *cobra.Command {
	var private bool
	cmd := &cobra.Command{
		Use:   "buckets",
		Short: "Given a terraform (images) plan, find all packages and their build deps",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if private {
				return buckets(ctx, []string{"os", "extra-packages", "enterprise-packages"})
			} else {
				return buckets(ctx, []string{"os", "extra-packages"})
			}
		},
	}
	cmd.Flags().BoolVar(&private, "private", false, "Set for images-private (includes enterprise-packages)")
	return cmd
}

// we'll want this to probably write things out to files, but for now this
// is just printing all origins that are reachable via the input plan
//
// that's a little janky because we co-mingle multiple repos, but we should
// split out origins based on the dir to separate files
func buckets(ctx context.Context, dirs []string) error {
	cache := apk.NewCache(true)

	buildRepos := map[string][]string{
		"os":                  []string{dirToRepo["os"]},
		"extra-packages":      []string{dirToRepo["os"], dirToRepo["extra-packages"]},
		"enterprise-packages": []string{dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	pkgss, err := dirToPackages(ctx, true)
	if err != nil {
		return err
	}

	pkgs := map[string]*config.Configuration{}
	cfgToDir := map[*config.Configuration]string{}

	for _, dir := range dirs {
		for pkg, cfg := range pkgss[dir] {
			pkgs[pkg] = cfg
			cfgToDir[cfg] = dir
		}
	}

	packages := map[string]config.Package{}
	subpackages := map[string]config.Subpackage{}
	origins := map[string]*config.Configuration{}

	missing := map[string]string{}
	missingDeps := map[string]string{}
	failed := map[string]error{}
	old := map[string]error{}

	// all apko_build configs from plan on stdin
	configs, err := walk(ctx, os.Stdin)
	if err != nil {
		return err
	}

	// set of pkg=ver lines from configs mapped back to (one) addr for nicer errors
	pkgvers := map[string]string{}
	for addr, cfg := range configs {
		for _, pkgver := range cfg.Contents.Packages {
			pkgvers[pkgver] = addr
		}
	}

	// Read every pkg=ver line and
	for line, addr := range pkgvers {
		pkg, ver, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("unexpected line: %q", line)
		}

		cfg, ok := pkgs[pkg]
		if !ok {
			missing[line] = addr
		} else {
			if got, want := cfg.Package.FullVersion(), ver; got != want {
				old[pkg+"-"+got] = fmt.Errorf("%s in %s", line, addr)
				continue
			}
			if cfg.Package.Name == pkg {
				packages[line] = cfg.Package
			} else {
				for _, subpkg := range cfg.Subpackages {
					if subpkg.Name == pkg {
						subpackages[line] = subpkg
					}
				}
			}
			origins[cfg.Package.Name] = cfg
		}
	}

	log.Printf("origins: %d", len(origins))

	done := map[string]struct{}{}
	for origin := range origins {
		done[origin] = struct{}{}
	}

	todo := maps.Clone(origins)

	// Each outer loop will lock the build environment for each origin in todo,
	// and add any new origins we discover to the next round.
	// This iteratively walks build dependencies until we bottom out.
	for {
		// Collect any newly discovered origins.
		deps := map[string]*config.Configuration{}

		var (
			g  errgroup.Group
			mu sync.Mutex
		)
		g.SetLimit(runtime.GOMAXPROCS(0))

		for next, cfg := range todo {
			g.Go(func() error {
				// Solve the build environment.
				more, err := lock(ctx, cfg, cache, buildRepos[cfgToDir[cfg]])
				if err != nil {
					mu.Lock()
					defer mu.Unlock()

					failed[next] = err
					done[next] = struct{}{}
					return nil
				}

				// Collect all the packages, mapping the package name to its build config.
				names := make(map[string]*config.Configuration, len(more))
				for _, line := range more {
					pkg, _, ok := strings.Cut(line, "=")
					if !ok {
						return fmt.Errorf("unexpected line: %q", line)
					}

					cfg, ok := pkgs[pkg]
					if !ok {
						mu.Lock()
						missingDeps[line] = next
						mu.Unlock()
					} else {
						names[cfg.Package.Name] = cfg
					}
				}

				mu.Lock()
				defer mu.Unlock()

				// Update our origins mapping with all those packages.
				// If we haven't already processed it, add it to our todo list.
				for name, cfg := range names {
					origins[name] = cfg

					if _, ok := done[cfg.Package.Name]; !ok {
						deps[name] = cfg
					}
				}

				done[next] = struct{}{}

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		if len(deps) == 0 {
			break
		}

		// Show some progress on each loop.
		log.Printf("added %d deps", len(deps))

		todo = deps
	}

	// log a bunch of unexpected things, we'll eventually need to fix these
	if len(missing) != 0 {
		log.Printf("images with unguarded packages:")
		for _, pkg := range slices.Sorted(maps.Keys(missing)) {
			log.Printf("  %s: %s", pkg, missing[pkg])
		}
	}

	if len(missingDeps) != 0 {
		log.Printf("no longer built from source packages:")
		for _, pkg := range slices.Sorted(maps.Keys(missingDeps)) {
			log.Printf("  %s: %s", missingDeps[pkg], pkg)
		}
	}

	if len(failed) != 0 {
		log.Printf("failed to build packages:")
		for pkg, err := range failed {
			log.Printf("  %s: %v", pkg, err)
		}
	}

	log.Printf("origins: %d", len(origins))
	log.Printf("packages: %d", len(packages))

	for origin := range origins {
		fmt.Println(origin)
	}

	return nil
}

func lock(ctx context.Context, c *config.Configuration, cache *apk.Cache, apkRepos []string) ([]string, error) {
	// Work around LockImageConfiguration assuming multi-arch.
	c.Environment.Archs = []apko_types.Architecture{"x86_64"}

	opts := []apko_build.Option{apko_build.WithImageConfiguration(c.Environment),
		apko_build.WithExtraBuildRepos(apkRepos),
		// TODO: multi-arch
		apko_build.WithArch("x86_64"),
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

// implementation of this ported from github.com/jonjohnsonjr/tfimages
func walk(ctx context.Context, in io.Reader) (map[string]*types.ImageConfiguration, error) {
	w := walker{
		configs: map[string]*types.ImageConfiguration{},
	}

	dec := json.NewDecoder(in)

	for i := 1; true; i++ {
		var p tfjson.Plan
		if err := dec.Decode(&p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if err := w.walkModules(p.PlannedValues.RootModule); err != nil {
			return nil, err
		}

		log.Printf("parsed %d plan files and found %d apko_builds", i, len(w.configs))
	}

	return w.configs, nil
}

type walker struct {
	// addr -> config
	configs map[string]*types.ImageConfiguration
}

func (w *walker) walkModules(m *tfjson.StateModule) error {
	for _, r := range m.Resources {
		if r.Type == "apko_build" {
			if strings.HasSuffix(r.Address, ".sandbox") {
				log.Printf("Skipping sandbox apko_build: %s", r.Address)
				continue
			}

			config, ok := r.AttributeValues["config"]
			if !ok {
				return fmt.Errorf("missing config: %s", r.Address)
			}

			b, err := json.Marshal(config)
			if err != nil {
				return fmt.Errorf("marshal: %w", err)
			}

			var ic types.ImageConfiguration
			if err := json.Unmarshal(b, &ic); err != nil {
				return fmt.Errorf("unmarshal: %w", err)
			}

			w.configs[r.Address] = &ic
		}
	}

	for _, c := range m.ChildModules {
		if err := w.walkModules(c); err != nil {
			return err
		}
	}

	return nil
}
