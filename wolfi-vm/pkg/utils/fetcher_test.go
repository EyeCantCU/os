//go:build withauth

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"os"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
)

var arches = []types.Architecture{types.ParseArchitecture("x86_64"), types.ParseArchitecture("aarch64")}

func TestFetchKernel(t *testing.T) {
	destDir := t.TempDir()

	for _, arch := range arches {
		t.Run(arch.ToAPK(), func(t *testing.T) {
			out, err := FetchKernel(destDir, arch.ToAPK())
			if err != nil {
				t.Fatalf("FetchKernel() = %v", err)
			}

			if _, err := os.Stat(out); err != nil {
				t.Fatalf("os.Stat() failed with %v", err)
			}
		})
	}
}

func TestFetchBios(t *testing.T) {
	destDir := t.TempDir()

	for _, arch := range arches {
		t.Run(arch.ToAPK(), func(t *testing.T) {
			out, err := FetchBios(destDir, arch.ToAPK())
			if err != nil {
				t.Fatalf("FetchBios() = %v", err)
			}

			if _, err := os.Stat(out); err != nil {
				t.Fatalf("os.Stat() failed with %v", err)
			}
		})
	}
}
