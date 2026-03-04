package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chainguard.dev/apko/pkg/apk/apk"
	"chainguard.dev/apko/pkg/build/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func compileCmd() *cobra.Command {
	var (
		arch string
		lock bool
	)

	cmd := &cobra.Command{
		Use:   "compile <package>",
		Short: "Compile a melange package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := strings.TrimSuffix(args[0], ".yaml")
			return compile(cmd.Context(), pkg, arch, lock)
		},
	}

	cmd.Flags().StringVar(&arch, "arch", "x86_64", "Architecture to compile for")
	cmd.Flags().BoolVar(&lock, "lock", false, "Whether to lock config")

	return cmd
}

func compile(ctx context.Context, pkg, arch string, lock bool) error {
	// Normalize so we ignore the dir and .yaml if provided.
	pkg = strings.TrimSuffix(filepath.Base(pkg), ".yaml")

	dir, err := findPackage(pkg)
	if err != nil {
		return fmt.Errorf("finding package %q: %w", pkg, err)
	}

	c, err := compilePkgConfig(ctx, filepath.Join(dir, pkg), types.ParseArchitecture(arch))
	if err != nil {
		return fmt.Errorf("compiling %q: %w", pkg, err)
	}

	// Build the repo list based on which directory the package lives in.
	buildRepos := map[string][]string{
		"os":                  {dirToRepo["os"]},
		"extra-packages":      {dirToRepo["os"], dirToRepo["extra-packages"]},
		"enterprise-packages": {dirToRepo["os"], dirToRepo["extra-packages"], dirToRepo["enterprise-packages"]},
	}

	repos := buildRepos[dir]

	c.Environment.Contents.BuildRepositories = repos

	if lock {
		locked, err := lockBuildDependencies(ctx, c, apk.NewCache(true), repos, arch)
		if err != nil {
			return fmt.Errorf("locking dependencies for %s: %w", pkg, err)
		}

		c.Environment.Contents.Packages = locked
	}

	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2) // To align with `yam` a little better.

	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	return nil
}
