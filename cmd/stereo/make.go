package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "helpers for makefile",
	}

	for _, subcmd := range []string{"package", "test", "debug", "test-debug"} {
		cmd.AddCommand(&cobra.Command{
			Use:   subcmd,
			Short: fmt.Sprintf("runs make %s/* in the right place", subcmd),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runMake(cmd.Context(), subcmd, args[0])
			},
		})
	}

	cmd.AddCommand(targetsCmd())

	return cmd
}

func targetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "targets",
		Short: "List package targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			seen := map[string]struct{}{}
			for _, dir := range precedence {
				matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
				if err != nil {
					return err
				}

				for _, match := range matches {
					target := strings.TrimSuffix(filepath.Base(match), ".yaml")
					if _, ok := seen[target]; ok {
						continue
					}

					seen[target] = struct{}{}
					fmt.Println(target)
				}
			}

			return nil
		},
	}
}

var precedence []string = []string{
	"os",
	"extra-packages",
	"enterprise-packages",
}

var dirToKeys map[string]string = map[string]string{
	"os":                  "local-melange.rsa.pub",
	"extra-packages":      "local-melange-extra.rsa.pub",
	"enterprise-packages": "local-melange-enterprise.rsa.pub",
}

func findPackage(pkg string) (string, error) {
	var errs []error
	for _, dir := range precedence {
		_, err := os.Stat(filepath.Join(dir, pkg) + ".yaml")
		if err == nil {
			return dir, nil
		}

		errs = append(errs, err)
	}

	return "", errors.Join(errs...)
}

func keyInit(ctx context.Context, dir, key string) error {
	cmd := exec.CommandContext(ctx, "make", key)

	cmd.Dir = dir

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runMake(ctx context.Context, subcmd, pkg string) error {
	dir, err := findPackage(pkg)
	if err != nil {
		return err
	}

	args := []string{}

	log.Printf("found %s in %s", pkg, dir)

	args = append(args, path.Join(subcmd, pkg))

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = dir

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = slices.Clone(os.Environ())

	extra := os.Getenv("MELANGE_EXTRA_OPTS")
	opts := []string{extra}

	for _, sub := range precedence {
		if sub == dir {
			break
		}

		if err := keyInit(ctx, sub, strings.TrimSuffix(dirToKeys[sub], ".pub")); err != nil {
			return fmt.Errorf("keyInit(%q): %w", sub, err)
		}

		opts = append(opts, fmt.Sprintf("--repository-append ../%s/packages", sub))
		opts = append(opts, fmt.Sprintf("--keyring-append ../%s/%s", sub, dirToKeys[sub]))
	}

	sourceDir := filepath.Join(dir, pkg)
	cfg, err := compilePkgConfig(ctx, sourceDir)
	if err != nil {
		return err
	}

	extra = strings.Join(opts, " ")

	cmd.Env = append(cmd.Env, fmt.Sprintf("MELANGE_EXTRA_OPTS=%s", extra))

	cleanup := func() {
		if err := cleanupTokens(dir, pkg); err != nil {
			log.Printf("failed to clean tokens for %s: %v", pkg, err)
		}
	}
	defer cleanup()

	if err := ensureTokens(ctx, cfg, sourceDir, subcmd); err != nil {
		return err
	}

	return cmd.Run()
}
