//go:build withauth

/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"os"
	"testing"
)

func TestFetchKernel(t *testing.T) {
	destDir := t.TempDir()

	out, err := FetchKernel(destDir)
	if err != nil {
		t.Fatalf("FetchKernel() = %v", err)
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("os.Stat() failed with %v", err)
	}
}

func TestFetchBios(t *testing.T) {
	destDir := t.TempDir()

	out, err := FetchBios(destDir)
	if err != nil {
		t.Fatalf("FetchKernel() = %v", err)
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("os.Stat() failed with %v", err)
	}
}
