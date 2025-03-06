//go:build withauth

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"chainguard.dev/apko/pkg/apk/auth"
	apkfs "chainguard.dev/apko/pkg/apk/fs"
	"chainguard.dev/apko/pkg/build"
	"chainguard.dev/apko/pkg/build/types"
	"gopkg.in/yaml.v3"
)

func buildImage(t *testing.T, destDir string) string {
	// We should comfortably be able to convert all of these images
	// in under a minute.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	b, err := os.ReadFile("../../configs/generic.yaml")
	if err != nil {
		t.Fatalf("os.ReadFile() failed with %v", err)
	}
	var ic types.ImageConfiguration
	dec := yaml.NewDecoder(bytes.NewBuffer(b))
	dec.KnownFields(true)
	if err := dec.Decode(&ic); err != nil {
		t.Fatalf("failed to parse image configuration: %v", err)
	}

	fs := apkfs.DirFS(t.TempDir(), apkfs.WithCreateDir())
	arch := types.ParseArchitecture(runtime.GOARCH)
	bc, err := build.New(ctx, fs,
		build.WithAuthenticator(auth.CGRAuth{}),
		build.WithArch(arch),
		build.WithImageConfiguration(ic),
	)
	if err != nil {
		t.Fatalf("build.New() failed with %v", err)
	}

	_, layer, err := bc.BuildLayer(ctx)
	if err != nil {
		t.Fatalf("bc.BuildLayer() failed with %v", err)
	}

	ucl, err := layer.Compressed()
	if err != nil {
		t.Fatalf("layer.Uncompressed() failed with %v", err)
	}

	outFile, err := os.Create(filepath.Join(destDir, "apko.tar.gz"))
	if err != nil {
		t.Fatalf("os.Create() failed with %v", err)
	}

	_, err = io.Copy(outFile, ucl)
	if err != nil {
		t.Fatalf("io.Copy() failed with %v", err)
	}

	return outFile.Name()
}

func TestCreateAttestation(t *testing.T) {
	destDir := t.TempDir()
	defer os.RemoveAll(destDir)

	apkoTar := buildImage(t, destDir)

	sbom, err := CreateAttestation(context.Background(), apkoTar)
	if err != nil {
		t.Fatalf("CreateAttestation() failed with %v", err)
	}

	_, err = os.Stat(sbom)
	if err != nil {
		t.Fatalf("CreateAttestation() did not create sbom file with %v", err)
	}
}
