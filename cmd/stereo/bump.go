package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chainguard.dev/melange/pkg/config"
	"github.com/spf13/cobra"
)

const epochPattern = `epoch: %d`

type bumpOptions struct {
	dryRun bool
}

func bumpCmd() *cobra.Command {
	opts := bumpOptions{}
	cmd := &cobra.Command{
		Use:     "bump config[.yaml] [config[.yaml]...]",
		Short:   "Bumps the epoch field in melange configuration files",
		Example: "stereo bump openssh.yaml perl",
		Long: `Bumps the epoch field in melange configuration files

The bump subcommand increments epoch numbers in package config files.

stereo bump can take a filename, a package or a file glob, increasing
the version in each matching configuration file:

    stereo bump zlib.yaml
    stereo bump openssl
    stereo bump lib*.yaml

You can use --dry-run to see which versions will be bumped without
modifying anything in the filesystem.

`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 0 {
				cmd.Help() //nolint:errcheck
				return fmt.Errorf("not enough arguments")
			}
			files := []string{}
			var errs []error
			for _, fname := range args {
				if strings.Contains(fname, "*") {
					for _, dir := range precedence {
						m, err := filepath.Glob(filepath.Join(dir, fname))
						if err != nil {
							errs = append(errs, fmt.Errorf("%s: %w", fname, err))
							break
						}

						files = append(files, m...)
					}

					continue
				}

				pkg := strings.TrimSuffix(fname, ".yaml")
				dir, err := findPackage(pkg)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", fname, err))
					continue
				}

				files = append(files, filepath.Join(dir, pkg)+".yaml")
			}

			if opts.dryRun {
				fmt.Fprint(os.Stderr, "dry-run: not writing data\n")
			}

			for _, f := range files {
				if err := bumpEpoch(ctx, opts, f); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "don't change anything, just print what would be done")

	return cmd
}

func bumpEpoch(ctx context.Context, opts bumpOptions, path string) error {
	cfg, err := config.ParseConfiguration(ctx, path)
	if err != nil {
		return fmt.Errorf("unable to parse configuration at %q: %w", path, err)
	}

	fmt.Fprintf(
		os.Stderr, "bumping %s-%s-%d in %s to epoch %d\n", cfg.Package.Name,
		cfg.Package.Version, cfg.Package.Epoch, path, cfg.Package.Epoch+1,
	)

	if opts.dryRun {
		return nil
	}

	original, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}

	scanner := bufio.NewScanner(original)
	scanner.Split(bufio.ScanLines)
	newFile := []string{}
	found := false
	old := fmt.Sprintf(epochPattern, cfg.Package.Epoch)
	for scanner.Scan() {
		line := scanner.Text()
		nocomment, _, _ := strings.Cut(line, "#")
		if strings.TrimSpace(nocomment) == old {
			found = true
			newFile = append(
				newFile, strings.ReplaceAll(line, old, fmt.Sprintf(epochPattern, cfg.Package.Epoch+1)),
			)
		} else {
			newFile = append(newFile, line)
		}
	}
	original.Close()

	if !found {
		return fmt.Errorf("unable to find epoch tag in yaml config")
	}

	if err := os.WriteFile(
		path, []byte(strings.Join(newFile, "\n")+"\n"), os.FileMode(0o644),
	); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
