/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"context"
	"fmt"
	"os"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/apko/pkg/cpio"
	"chainguard.dev/apko/pkg/tarfs"
)

// CreateCpio is modeled after the apko build-cpio command.
func CreateCpio(ctx context.Context, dest string, opts ...build.Option) error {
	wd, err := os.MkdirTemp("", "apko-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer os.RemoveAll(wd)

	bc, err := build.New(ctx, tarfs.New(), opts...)
	if err != nil {
		return err
	}

	_, layer, err := bc.BuildLayer(ctx)
	if err != nil {
		return fmt.Errorf("failed to build layer image: %w", err)
	}

	// Create the CPIO file, and set up a deduplicating writer
	// to produce the gzip-compressed CPIO archive.
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	return cpio.FromLayer(layer, f)
}

// CreateCpio is modeled after the apko build-minirootfs command.
func CreateTar(ctx context.Context, config string, targetArch string) (string, error) {
	buildconf, err := os.Open(config)
	if err != nil {
		return "", fmt.Errorf("failed to open apko yaml: %w", err)
	}
	defer buildconf.Close()

	opts := []build.Option{
		build.WithConfig(config, []string{}),
		build.WithArch(types.ParseArchitecture(targetArch)),
	}

	wd, err := os.MkdirTemp("", "apko-*")
	if err != nil {
		return "", fmt.Errorf("failed to create working directory: %w", err)
	}

	defer os.RemoveAll(wd)

	// use tarfs instead of apkofs.DirFS as it preserves reproducibility.
	bc, err := build.New(ctx, tarfs.New(), opts...)
	if err != nil {
		return "", err
	}

	layer, _, err := bc.BuildLayer(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build layer image: %w", err)
	}

	return layer, nil
}
