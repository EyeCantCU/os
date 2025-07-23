package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"chainguard.dev/melange/pkg/build"
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var dirToRepo map[string]string = map[string]string{
	// TODO: These aren't all apk.cgr.dev because there is a
	// meaningful diff for build dependencies of some packages.
	"os":                  "https://packages.wolfi.dev/os",
	"extra-packages":      "https://packages.cgr.dev/extras",
	"enterprise-packages": "https://apk.cgr.dev/chainguard-private",
}

func main() {
	root := &cobra.Command{
		Use:          "stereo",
		Short:        "Manage the stereo repo",
		SilenceUsage: true,
	}

	root.AddCommand(bucketsCmd())
	root.AddCommand(lintCmd())
	root.AddCommand(archiveCmd())
	root.AddCommand(buildDepsCmd())
	root.AddCommand(imageDependenciesCmd())
	root.AddCommand(vmDependenciesCmd())
	root.AddCommand(transitionCmd())

	if err := root.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func lintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Find duplicate package names",
		RunE: func(cmd *cobra.Command, args []string) error {
			return lint(cmd.Context())
		},
	}
}

func lint(ctx context.Context) error {
	pkgss, err := dirToPackages(ctx)
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

func dirToPackages(ctx context.Context) (map[string]map[string]*config.Configuration, error) {
	pkgss := map[string]map[string]*config.Configuration{}

	var g errgroup.Group
	for dir := range dirToRepo {
		g.Go(func() error {
			local := fmt.Sprintf("./%s", dir)
			pipelines := fmt.Sprintf("./%s/pipelines/", dir)
			pkgs, err := NewPackages(ctx, os.DirFS(dir), local, pipelines)
			if err != nil {
				return fmt.Errorf("walking %s: %w", dir, err)
			}
			pkgss[dir] = pkgs
			return nil
		})
	}

	return pkgss, g.Wait()
}

func NewPackages(ctx context.Context, fsys fs.FS, dirPath, pipelineDir string) (map[string]*config.Configuration, error) {
	pkgs := map[string]*config.Configuration{}

	var (
		g    errgroup.Group
		mu   sync.Mutex
		errs []error
	)

	g.SetLimit(runtime.GOMAXPROCS(0))

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip anything in .github/ and .git/
		if path == ".github" {
			if d.Type().IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == ".git" {
			if d.Type().IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// .yam.yaml, .melange.k8s.yaml, .golangci.yaml, .pre-commit-config.yaml, etc.
		if d.Type().IsRegular() && strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Skip any file that isn't a yaml file
		if !d.Type().IsRegular() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		if filepath.Dir(path) != "." && !strings.HasSuffix(path, ".melange.yaml") {
			return nil
		}

		g.Go(func() error {
			c, err := config.ParseConfiguration(ctx, path, config.WithFS(fsys))
			if err != nil {
				return err
			}

			// Resolve all `uses` used by the pipeline. This updates the set of
			// .environment.contents.packages so the next block can include those as build deps.
			build := &build.Build{
				PipelineDirs:  []string{pipelineDir},
				Configuration: c,
			}
			if err := build.Compile(ctx); err != nil {
				return fmt.Errorf("compiling build: %w", err)
			}
			c.Environment = build.Configuration.Environment

			mu.Lock()
			defer mu.Unlock()

			if _, ok := pkgs[c.Package.Name]; ok {
				errs = append(errs, fmt.Errorf("conflict: %q in %s.yaml", c.Package.Name, c.Package.Name))
			}

			pkgs[c.Package.Name] = c
			for i := range c.Subpackages {
				subpkg := c.Subpackages[i]

				if _, ok := pkgs[subpkg.Name]; ok {
					errs = append(errs, fmt.Errorf("conflict: %q in %s.yaml", subpkg.Name, c.Package.Name))
				}

				pkgs[subpkg.Name] = c
			}

			return nil
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking filesystem: %w", err)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return pkgs, errors.Join(errs...)
}
