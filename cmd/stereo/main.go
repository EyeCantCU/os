package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"chainguard.dev/melange/pkg/build"
	"chainguard.dev/melange/pkg/config"
	"golang.org/x/sync/errgroup"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "lint" {
		log.Fatalf("usage: %s lint", os.Args[0])
	}
	if err := lint(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func lint(ctx context.Context) error {
	var errs []error

	wolfi, err := NewPackages(ctx, os.DirFS("os"), "./os", "./os/pipelines/")
	if err != nil {
		errs = append(errs, fmt.Errorf("wolfi: %w", err))
	}

	extras, err := NewPackages(ctx, os.DirFS("extra-packages"), "./extra-packages", "./extra-packages/pipelines/")
	if err != nil {
		errs = append(errs, fmt.Errorf("extras: %w", err))
	}

	enterprise, err := NewPackages(ctx, os.DirFS("enterprise-packages"), "./enterprise-packages", "./enterprise-packages/pipelines/")
	if err != nil {
		errs = append(errs, fmt.Errorf("enterprise: %w", err))
	}

	seen := map[string]*config.Configuration{}

	for _, repo := range []map[string]*config.Configuration{wolfi, extras, enterprise} {
		for pkg, cfg := range repo {
			if got, ok := seen[pkg]; ok {
				errs = append(errs, fmt.Errorf("conflict: %q in %s.yaml and %s.yaml", pkg, got.Package.Name, cfg.Package.Name))
			}
			seen[pkg] = cfg
		}
	}

	// log.Printf("wolfi has %d packages", len(wolfi))
	// log.Printf("extras has %d packages", len(extras))
	// log.Printf("enterprise has %d packages", len(enterprise))

	return errors.Join(errs...)
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
			c.Environment.Contents.Packages = build.Configuration.Environment.Contents.Packages

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
