package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"chainguard.dev/apko/pkg/apk/apk"
	"chainguard.dev/apko/pkg/apk/auth"
	apko_build "chainguard.dev/apko/pkg/build"
	apko_types "chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/melange/pkg/build"
	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// archToApkoArch maps melange architecture names to apko architecture names
func archToApkoArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return arch
	}
}

// supportsApkoArchitecture checks if an apko configuration supports the given architecture
func supportsApkoArchitecture(cfg *apko_types.ImageConfiguration, arch string) bool {
	// If no archs is specified, configuration supports all architectures
	if len(cfg.Archs) == 0 {
		return true
	}

	apkoArch := archToApkoArch(arch)
	// Check if the architecture is in the archs list
	for _, targetArch := range cfg.Archs {
		if string(targetArch) == apkoArch {
			return true
		}
	}
	return false
}

const (
	OsRepo                 = "https://apk.cgr.dev/chainguard"
	ExtraPackagesRepo      = "https://apk.cgr.dev/extra-packages"
	EnterprisePackagesRepo = "https://apk.cgr.dev/chainguard-private"
)

var dirToRepo map[string]string = map[string]string{
	"os":                  OsRepo,
	"extra-packages":      ExtraPackagesRepo,
	"enterprise-packages": EnterprisePackagesRepo,
}

// dirToWithdrawnRepo creates a mapping from directory names to withdrawn repository paths
func dirToWithdrawnRepo(withdrawnDir string) map[string]string {
	absWithdrawnDir, err := filepath.Abs(withdrawnDir)
	if err != nil {
		log.Printf("Warning: Could not get absolute path for %s, using relative path", withdrawnDir)
		absWithdrawnDir = withdrawnDir
	}

	return map[string]string{
		"os":                  filepath.Join(absWithdrawnDir, "os"),
		"extra-packages":      filepath.Join(absWithdrawnDir, "extra-packages"),
		"enterprise-packages": filepath.Join(absWithdrawnDir, "enterprise-packages"),
	}
}

func main() {
	root := &cobra.Command{
		Use:          "stereo",
		Short:        "Manage the stereo repo",
		SilenceUsage: true,
	}

	root.AddCommand(archiveCmd())
	root.AddCommand(bucketsCmd())
	root.AddCommand(buildDepsCmd())
	root.AddCommand(bumpCmd())
	root.AddCommand(checkCmd())
	root.AddCommand(editCmd())
	root.AddCommand(imageDependenciesCmd())
	root.AddCommand(lintCmd())
	root.AddCommand(makeCmd())
	root.AddCommand(impactCmd())
	root.AddCommand(seedDependenciesCmd())
	root.AddCommand(unguardedCmd())
	root.AddCommand(outdatedCmd())
	root.AddCommand(vmDependenciesCmd())
	root.AddCommand(withdrawCmd())
	root.AddCommand(transitionCmd())

	if err := root.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func dirToPackages(ctx context.Context, subpackages bool) (map[string]map[string]*config.Configuration, error) {
	pkgss := map[string]map[string]*config.Configuration{}

	var (
		mu sync.Mutex
		g  errgroup.Group
	)
	for dir := range dirToRepo {
		g.Go(func() error {
			local := fmt.Sprintf("./%s", dir)
			pipelines := fmt.Sprintf("./%s/pipelines/", dir)
			var pkgs map[string]*config.Configuration
			var err error
			if subpackages {
				pkgs, err = NewPackages(ctx, os.DirFS(dir), local, pipelines)
			} else {
				pkgs, err = NewOrigins(ctx, os.DirFS(dir), local, pipelines)
			}
			if err != nil {
				return fmt.Errorf("walking %s: %w", dir, err)
			}

			mu.Lock()
			defer mu.Unlock()

			pkgss[dir] = pkgs
			return nil
		})
	}

	return pkgss, g.Wait()
}

func dirToOrigins(ctx context.Context) (map[string]map[string]*config.Configuration, error) {
	pkgss := map[string]map[string]*config.Configuration{}

	var (
		mu sync.Mutex
		g  errgroup.Group
	)
	for dir := range dirToRepo {
		g.Go(func() error {
			local := fmt.Sprintf("./%s", dir)
			pipelines := fmt.Sprintf("./%s/pipelines/", dir)
			pkgs, err := NewOrigins(ctx, os.DirFS(dir), local, pipelines)
			if err != nil {
				return fmt.Errorf("walking %s: %w", dir, err)
			}

			mu.Lock()
			defer mu.Unlock()

			pkgss[dir] = pkgs
			return nil
		})
	}

	return pkgss, g.Wait()
}

// NewPackages returns map of every package to its config, including subpackages.
// See NewOrigins if you only care about unique build environments.
func NewPackages(ctx context.Context, fsys fs.FS, dirPath, pipelineDir string) (map[string]*config.Configuration, error) {
	origins, err := NewOrigins(ctx, fsys, dirPath, pipelineDir)
	if err != nil {
		return nil, err
	}

	pkgs := maps.Clone(origins)
	var errs []error
	for _, c := range origins {
		for i := range c.Subpackages {
			subpkg := c.Subpackages[i]

			if other, ok := pkgs[subpkg.Name]; ok {
				errs = append(errs, fmt.Errorf("conflict: %s: %q in %s.yaml and %s.yaml", dirPath, subpkg.Name, c.Package.Name, other.Package.Name))
			}

			pkgs[subpkg.Name] = c
		}
	}

	return pkgs, errors.Join(errs...)
}

// NewOrigins returns a map of main package to its config.
func NewOrigins(ctx context.Context, fsys fs.FS, dirPath, pipelineDir string) (map[string]*config.Configuration, error) {
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
				return fmt.Errorf("parsing %q: %w", path, err)
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

// lock dependencies for a Melange confiugration
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
		config := &bytes.Buffer{}
		if err := json.NewEncoder(config).Encode(c.Environment); err != nil {
			return nil, fmt.Errorf("encoding %s: %w", c.Name(), err)
		}
		return nil, fmt.Errorf("unable to lock image configuration: %w\nimage config:\n%s", err, config.String())
	}

	locked, ok := configs["index"]
	if !ok {
		return nil, errors.New("missing locked config")
	}

	return locked.Contents.Packages, nil
}

// lock build dependencies for an Image configuration
func lockImageDependencies(ctx context.Context, cfg *apko_types.ImageConfiguration, cache *apk.Cache, apkRepos []string, arch string) ([]string, error) {
	// Work around LockImageConfiguration assuming multi-arch.
	cfg.Archs = []apko_types.Architecture{apko_types.Architecture(arch)}

	opts := []apko_build.Option{
		apko_build.WithImageConfiguration(*cfg),
		apko_build.WithExtraBuildRepos(apkRepos),
		apko_build.WithArch(apko_types.Architecture(arch)),
		// TODO: Allow offline.
		apko_build.WithCache("", false, cache),
		// TODO: Fix that.
		apko_build.WithIgnoreSignatures(true),
	}

	configs, _, err := apko_build.LockImageConfiguration(ctx, *cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to lock image configuration: %w", err)
	}

	locked, ok := configs["index"]
	if !ok {
		return nil, fmt.Errorf("missing locked config")
	}

	return locked.Contents.Packages, nil
}

// format a time duration as days
func formatDurationInDays(d time.Duration) string {
	days := d.Hours() / 24
	if days >= 1 {
		return fmt.Sprintf("%.0f days", days)
	}
	return d.String() // fallback for sub-day durations
}

// fetchAPKIndex downloads and parses an APKINDEX.tar.gz file from a repository
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

// loadLocalAPKIndex loads an APKINDEX from a local file
func loadLocalAPKIndex(indexPath string) (*apk.APKIndex, error) {
	file, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("opening local APKINDEX file %s: %w", indexPath, err)
	}
	defer file.Close()

	return apk.IndexFromArchive(file)
}
