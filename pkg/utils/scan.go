/*
Copyright 2024 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/cataloging/filecataloging"
	"github.com/anchore/syft/syft/cataloging/pkgcataloging"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/directorysource"
	"github.com/chainguard-dev/clog"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/wolfi-dev/wolfictl/pkg/sbom/catalogers"
	"github.com/wolfi-dev/wolfictl/pkg/tar"
)

func CreateAttestationFromLayer(ctx context.Context, layer v1.Layer) (*bytes.Buffer, error) {
	input, err := os.CreateTemp("", "layer-*.tar.gz")
	if err != nil {
		return nil, err
	}
	defer input.Close()

	defer os.Remove(input.Name())

	ul, err := layer.Compressed()
	if err != nil {
		return nil, err
	}
	defer ul.Close()

	_, err = io.Copy(input, ul)
	if err != nil {
		return nil, fmt.Errorf("failed to read layer: %w", err)
	}

	sbom, err := CreateAttestation(ctx, input.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to create sbom: %w", err)
	}

	sbomFile, err := os.Open(sbom)
	if err != nil {
		return nil, fmt.Errorf("failed to read sbom file: %w", err)
	}
	defer sbomFile.Close()

	var buf bytes.Buffer

	_, err = io.Copy(&buf, sbomFile)
	if err != nil {
		return nil, fmt.Errorf("failed to copy sbom file: %w", err)
	}

	return &buf, nil
}

func CreateAttestation(ctx context.Context, input string) (string, error) {
	clog.Info("creating syft attestation")

	tempDir, err := os.MkdirTemp("", "wolfictl-sbom-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		clog.Debug("cleaning up temp directory", "path", tempDir)
		_ = os.RemoveAll(tempDir)
	}()

	clog.Debug("created temp directory to unpack apko tar", "path", tempDir)

	file, err := os.Open(input)
	if err != nil {
		return "", fmt.Errorf("failed to create unpack apko tar: %w", err)
	}

	clog.Debug("unpacking apko tar", "path", tempDir)
	// Unpack tar to temp directory
	if err := tar.Untar(file, tempDir); err != nil {
		return "", fmt.Errorf("failed to unpack tar file: %w", err)
	}

	src, err := directorysource.New(
		directorysource.Config{
			Path: tempDir,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to create source from directory: %w", err)
	}
	clog.Debug("created Syft source from directory", "description", src.Describe())

	cfg := syft.DefaultCreateSBOMConfig().WithCatalogerSelection(
		pkgcataloging.NewSelectionRequest().WithDefaults(
			pkgcataloging.ImageTag,
			filecataloging.FileTag, // see https://github.com/anchore/syft/pull/3505 for context
		).WithRemovals(
			"sbom",
			// TODO consider how to turn it on https://github.com/chainguard-dev/internal-dev/issues/8731
			"elf-package",
		),
	).WithCatalogers(
		catalogers.AngularJSReference,
		catalogers.PipVendorReference,
		catalogers.WheelReference,
	)
	cfg.Parallelism = runtime.NumCPU()

	// Generate the SBOM
	sbom, err := syft.CreateSBOM(ctx, src, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create SBOM: %w", err)
	}
	// remove tmp path from metadata
	sbom.Source.Name = "wolfi-vm"
	sbom.Source.Metadata = source.FileMetadata{}

	// Normalize owner/group IDs, by default syft will
	// detect current UID/GID ownage for system files
	// this will normalize it back to root.
	// This will only do for files owned by the user
	// doing the scan, so others are not touched.
	uid := os.Getuid()
	gid := os.Getgid()
	for i := range sbom.Artifacts.FileMetadata {
		metadata := sbom.Artifacts.FileMetadata[i]
		if metadata.UserID == uid {
			metadata.UserID = 0
		}
		if metadata.GroupID == gid {
			metadata.GroupID = 0
		}
		sbom.Artifacts.FileMetadata[i] = metadata
	}

	// Encode the SBOM to Syft JSON format
	enc := syftjson.NewFormatEncoder()
	if enc == nil {
		return "", fmt.Errorf("failed to get JSON encoder")
	}

	outputPath := filepath.Join(
		filepath.Dir(input),
		"syft.sbom.json",
	)

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	// Write the SBOM JSON to a file
	err = enc.Encode(outputFile, *sbom)
	if err != nil {
		return "", fmt.Errorf("failed to encode SBOM: %w", err)
	}

	return outputPath, nil
}
