/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func WriteDiskGzip(rawDisk string, wc io.Writer) error {
	gzw := gzip.NewWriter(wc)
	f, err := os.Open(rawDisk)
	if err != nil {
		return fmt.Errorf("os.Open() failed with %w", err)
	}
	if _, err := io.Copy(gzw, f); err != nil {
		return fmt.Errorf("io.Copy() failed with %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("f.Close() failed with %w", err)
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("gzw.Close() failed with %w", err)
	}
	return nil
}

func WriteDiskTarGzip(ctx context.Context, rawDisk string, wc io.Writer) error {
	// Now convert things into the format GCP expects.
	// tar --format=oldgnu -Sczf $disk.tar.gz $(basename $disk)
	// nolint:gosec // We trust the paths we get here.
	tar := exec.CommandContext(ctx, "tar",
		// GCP expects the oldgnu format.
		"--format=oldgnu",
		"-C", filepath.Dir(rawDisk),
		// GCP expects a sparse tar file.
		"-Sc", filepath.Base(rawDisk),
	)
	gzw := gzip.NewWriter(wc)
	tar.Stdout = gzw
	tar.Stderr = os.Stderr
	if err := tar.Run(); err != nil {
		return fmt.Errorf("failed to run tar %w", err)
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("failed to close writer %w", err)
	}
	return nil
}

func WriteDiskVPCGzip(ctx context.Context, rawDisk string, wc io.Writer, extraArgs ...string) error {
	args := append([]string{
		"convert",
		"-f", "raw",
		"-O", "vpc",
	}, extraArgs...)

	vpcDisk := filepath.Join(filepath.Dir(rawDisk), "disk.vpc")
	args = append(args, rawDisk, vpcDisk)

	// Run the disk conversion.
	qic := exec.CommandContext(ctx, "qemu-img", args...)
	qic.Stdout = os.Stdout
	qic.Stderr = os.Stderr
	if err := qic.Run(); err != nil {
		return fmt.Errorf("failed to run qemu-img %w", err)
	}
	return WriteDiskGzip(vpcDisk, wc)
}
