// Copyright 2025 Chainguard, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package publisher

import (
	"os"
	"path/filepath"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
)

// Helper functions for test setup

// createTestOutputDir creates a test output directory with specified files
// The directory is created with a platform-prefixed name (e.g., "aws-test", "azure-test")
func createTestOutputDir(t *testing.T, platform string, files []string) string {
	parent := t.TempDir()
	// Create subdirectory with platform-prefixed name
	dirName := platform + "-test"
	dir := filepath.Join(parent, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", f, err)
		}
	}
	return dir
}

// createTestConfigDir creates configs/{configName}/build.yaml structure
func createTestConfigDir(t *testing.T, configName string) {
	configDir := filepath.Join("configs", configName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(configDir)
		// Remove parent configs directory if empty
		os.Remove("configs")
	})

	buildYaml := filepath.Join(configDir, "build.yaml")
	content := `contents:
  packages:
    - wolfi-base
`
	if err := os.WriteFile(buildYaml, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write build.yaml: %v", err)
	}
}

// TestLoadArtifactsFromDir tests loading VM artifacts (disk files, SBOMs, apko tarballs,
// secure boot files) from directories. Validates platform detection, config path resolution,
// and error handling for missing directories or files.
func TestLoadArtifactsFromDir(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(t *testing.T) string
		arch         types.Architecture
		platform     Platform
		wantErr      bool
		validateFunc func(t *testing.T, artifacts *Artifacts)
	}{
		{
			name: "all artifact types",
			setupFunc: func(t *testing.T) string {
				files := []string{
					"disk.raw",
					"disk.qcow2",
					"disk.vmdk",
					"disk.vhd",
					"disk.ova",
					"apko-x86_64.tar.gz",
					"sbom-x86_64.spdx.json",
					"syft.sbom.json",
					"uefi-data.json",
					"uefi-data.fd",
					"custom.auth",
				}
				return createTestOutputDir(t, "aws", files)
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformAWS,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				if artifacts.ApkoTarball == "" {
					t.Error("expected ApkoTarball to be set")
				}
				if len(artifacts.DiskRaw) != 1 {
					t.Errorf("expected 1 raw disk, got %d", len(artifacts.DiskRaw))
				}
				if len(artifacts.DiskQcow2) != 1 {
					t.Errorf("expected 1 qcow2 disk, got %d", len(artifacts.DiskQcow2))
				}
				if len(artifacts.DiskVmdk) != 1 {
					t.Errorf("expected 1 vmdk disk, got %d", len(artifacts.DiskVmdk))
				}
				if len(artifacts.DiskVhd) != 1 {
					t.Errorf("expected 1 vhd disk, got %d", len(artifacts.DiskVhd))
				}
				if len(artifacts.DiskOva) != 1 {
					t.Errorf("expected 1 ova disk, got %d", len(artifacts.DiskOva))
				}
				if len(artifacts.ApkoSBOMs) != 1 {
					t.Errorf("expected 1 apko SBOM, got %d", len(artifacts.ApkoSBOMs))
				}
				if artifacts.SyftSBOM == "" {
					t.Error("expected SyftSBOM to be set")
				}
				if len(artifacts.SecureBootFiles) != 3 {
					t.Errorf("expected 3 secure boot files, got %d", len(artifacts.SecureBootFiles))
				}
			},
		},
		{
			name: "minimal artifacts",
			setupFunc: func(t *testing.T) string {
				files := []string{
					"disk.raw",
					"apko.tar.gz",
				}
				return createTestOutputDir(t, "azure", files)
			},
			arch:     types.ParseArchitecture("aarch64"),
			platform: PlatformAzure,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				if artifacts.ApkoTarball == "" {
					t.Error("expected ApkoTarball to be set")
				}
				if len(artifacts.DiskRaw) != 1 {
					t.Errorf("expected 1 raw disk, got %d", len(artifacts.DiskRaw))
				}
				if artifacts.Arch != types.ParseArchitecture("aarch64") {
					t.Errorf("expected arch aarch64, got %v", artifacts.Arch)
				}
			},
		},
		{
			name: "no disk files - error",
			setupFunc: func(t *testing.T) string {
				files := []string{
					"apko.tar.gz",
					"sbom-x86_64.spdx.json",
				}
				return createTestOutputDir(t, "gcp", files)
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformGCP,
			wantErr:  true,
		},
		{
			name: "directory does not exist",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/directory"
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformAWS,
			wantErr:  true,
		},
		{
			name: "aws platform extraction",
			setupFunc: func(t *testing.T) string {
				files := []string{"disk.raw"}
				// Create with aws platform prefix directly
				return createTestOutputDir(t, "aws", files)
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformAWS,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				if artifacts.Platform != PlatformAWS {
					t.Errorf("expected platform AWS, got %v", artifacts.Platform)
				}
			},
		},
		{
			name: "multiple SBOMs",
			setupFunc: func(t *testing.T) string {
				files := []string{
					"disk.raw",
					"sbom-x86_64.spdx.json",
					"sbom-aarch64.spdx.json",
					"syft.sbom.json",
				}
				return createTestOutputDir(t, "qemu", files)
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformQEMU,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				if len(artifacts.ApkoSBOMs) != 2 {
					t.Errorf("expected 2 apko SBOMs, got %d", len(artifacts.ApkoSBOMs))
				}
				if artifacts.SyftSBOM == "" {
					t.Error("expected SyftSBOM to be set")
				}
			},
		},
		{
			name: "BuildConfigPath exists",
			setupFunc: func(t *testing.T) string {
				files := []string{"disk.raw"}
				dir := createTestOutputDir(t, "aws", files)
				// Rename from "aws-test" to "aws-image" for config matching
				awsImageDir := filepath.Join(filepath.Dir(dir), "aws-image")
				if err := os.Rename(dir, awsImageDir); err != nil {
					t.Fatalf("failed to rename dir: %v", err)
				}
				// Create the matching config file
				createTestConfigDir(t, "aws-image")
				return awsImageDir
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformAWS,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				expectedPath := filepath.Join("configs", "aws-image", "build.yaml")
				if artifacts.BuildConfigPath != expectedPath {
					t.Errorf("expected BuildConfigPath %s, got %s", expectedPath, artifacts.BuildConfigPath)
				}
			},
		},
		{
			name: "BuildConfigPath missing",
			setupFunc: func(t *testing.T) string {
				files := []string{"disk.raw"}
				dir := createTestOutputDir(t, "azure", files)
				// Rename from "azure-test" to "azure-nonexistent" (no config will exist for this)
				noConfigDir := filepath.Join(filepath.Dir(dir), "azure-nonexistent")
				if err := os.Rename(dir, noConfigDir); err != nil {
					t.Fatalf("failed to rename dir: %v", err)
				}
				return noConfigDir
			},
			arch:     types.ParseArchitecture("x86_64"),
			platform: PlatformAzure,
			wantErr:  false,
			validateFunc: func(t *testing.T, artifacts *Artifacts) {
				if artifacts.BuildConfigPath != "" {
					t.Errorf("expected BuildConfigPath to be empty, got %s", artifacts.BuildConfigPath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := tt.setupFunc(t)

			artifacts, err := LoadArtifactsFromDir(outputDir, tt.arch, tt.platform)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadArtifactsFromDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validateFunc != nil {
				tt.validateFunc(t, artifacts)
			}
		})
	}
}
