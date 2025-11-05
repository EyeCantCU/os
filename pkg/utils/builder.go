/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/oci"
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

	opts = append(opts, build.WithTempDir(wd))

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

// CreateTar is modeled after the apko build-minirootfs command.
// It creates a tar layer and generates SPDX SBOMs in the specified output directory.
// Returns the layer path, SBOM paths, and any error encountered.
func CreateTar(ctx context.Context, config string, targetArch string, sbomOutputDir string) (layerPath string, sbomPaths []string, err error) {
	buildconf, err := os.Open(config)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open apko yaml: %w", err)
	}
	defer buildconf.Close()

	opts := []build.Option{
		build.WithConfig(config, []string{}),
		build.WithArch(types.ParseArchitecture(targetArch)),
		build.WithSBOM(sbomOutputDir),
		build.WithSBOMFormats([]string{"spdx"}),
	}

	wd, err := os.MkdirTemp("", "apko-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create working directory: %w", err)
	}
	defer os.RemoveAll(wd)

	// use tarfs instead of apkofs.DirFS as it preserves reproducibility.
	bc, err := build.New(ctx, tarfs.New(), opts...)
	if err != nil {
		return "", nil, err
	}

	layerPath, layer, err := bc.BuildLayer(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build layer image: %w", err)
	}

	// Get build date epoch
	bde, err := bc.GetBuildDateEpoch()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get build date epoch: %w", err)
	}

	// Build an OCI image from the layers
	arch := types.ParseArchitecture(targetArch)
	img, err := oci.BuildImageFromLayers(ctx, bc.BaseImage(), []v1.Layer{layer}, bc.ImageConfiguration(), bde, arch)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build image for SBOM generation: %w", err)
	}

	// Generate SBOMs
	sboms, err := bc.GenerateImageSBOM(ctx, arch, img)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate SBOM: %w", err)
	}

	for _, sbom := range sboms {
		sbomPaths = append(sbomPaths, sbom.Path)
	}

	return layerPath, sbomPaths, nil
}
