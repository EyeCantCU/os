/*
Copyright 2024 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"archive/tar"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/cataloging/filecataloging"
	"github.com/anchore/syft/syft/cataloging/pkgcataloging"
	"github.com/anchore/syft/syft/format/syftjson"
	"github.com/anchore/syft/syft/source"
	"github.com/anchore/syft/syft/source/directorysource"
	"github.com/chainguard-dev/clog"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/wolfi-dev/wolfictl/pkg/sbom/catalogers"
)

func CreateAttestationFromLayer(ctx context.Context, layer v1.Layer) (io.Reader, error) {
	r, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return CreateAttestation(ctx, r)
}

func CreateAttestation(ctx context.Context, layer io.Reader) (io.Reader, error) {
	clog.Info("creating syft attestation")

	tempDir, err := os.MkdirTemp("", "wolfictl-sbom-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		clog.Debug("cleaning up temp directory", "path", tempDir)
		_ = os.RemoveAll(tempDir)
	}()

	clog.Debug("created temp directory to unpack apko tar", "path", tempDir)

	clog.Debug("unpacking apko tar", "path", tempDir)
	// Unpack tar to temp directory
	if err := untar(layer, tempDir); err != nil {
		return nil, fmt.Errorf("failed to unpack tar file: %w", err)
	}

	src, err := directorysource.New(
		directorysource.Config{
			Path: tempDir,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create source from directory: %w", err)
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
		return nil, fmt.Errorf("failed to create SBOM: %w", err)
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
		return nil, fmt.Errorf("failed to get JSON encoder")
	}

	var buf bytes.Buffer
	// Write the SBOM JSON to a file
	err = enc.Encode(&buf, *sbom)
	if err != nil {
		return nil, fmt.Errorf("failed to encode SBOM: %w", err)
	}

	return &buf, nil
}

func untar(src io.Reader, dst string) error {
	tr := tar.NewReader(src)

	// uncompress each element
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break // End of archive
		}
		if err != nil {
			return err
		}

		target, err := sanitizeArchivePath(dst, header.Name)
		// validate name against path traversal
		if err != nil {
			return err
		}

		// check the type
		switch header.Typeflag {
		// Create directories
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, os.ModePerm); err != nil {
					return err
				}
			}
		// Write out files
		case tar.TypeReg:
			// Ensure the parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
				return err
			}

			mode := header.Mode

			// Check if mode is within the range of a uint32
			if mode < 0 || mode > int64(^uint32(0)) {
				return fmt.Errorf("file mode out of range: %d", mode)
			}

			fileToWrite, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(mode))
			if err != nil {
				return err
			}

			if _, err := io.CopyN(fileToWrite, tr, header.Size); err != nil {
				return err
			}

			if err := fileToWrite.Close(); err != nil {
				return fmt.Errorf("failed to close file %s: %w", target, err)
			}
		}
	}
	return nil
}

// From https://github.com/securego/gosec/issues/324
func sanitizeArchivePath(d, t string) (string, error) {
	// Convert to forward slashes
	cleanedTarget := filepath.FromSlash(t)

	v := filepath.Join(d, cleanedTarget)
	cleanedBase := filepath.Clean(d)

	if strings.HasPrefix(v, cleanedBase) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}
