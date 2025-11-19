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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
)

// TestGeneratePredicates validates generation of all 4 predicate types
// (SPDX SBOM, Image Config, SLSA Provenance, VM Metadata) for attestation.
func TestGeneratePredicates(t *testing.T) {
	// Create temporary directory for test artifacts
	tmpDir := t.TempDir()

	// Create a test SBOM file
	sbomPath := filepath.Join(tmpDir, "sbom-test.spdx.json")
	sbomData := map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"name":        "test-sbom",
	}
	sbomJSON, err := json.Marshal(sbomData)
	if err != nil {
		t.Fatalf("failed to marshal test SBOM: %v", err)
	}
	if err := os.WriteFile(sbomPath, sbomJSON, 0644); err != nil {
		t.Fatalf("failed to write test SBOM: %v", err)
	}

	// Create a test build config file
	configPath := filepath.Join(tmpDir, "build.yaml")
	configData := `contents:
  packages:
    - wolfi-base
    - busybox
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Create test artifacts
	artifacts := &Artifacts{
		Arch:            types.ParseArchitecture("x86_64"),
		Platform:        "qemu",
		ApkoSBOMs:       []string{sbomPath},
		BuildConfigPath: configPath,
		DiskRaw:         []string{"/fake/disk.raw"},
	}

	// Generate predicates
	predicates, err := generatePredicates(artifacts)
	if err != nil {
		t.Fatalf("generatePredicates failed: %v", err)
	}

	// Verify we got the expected predicates
	if len(predicates) != 4 {
		t.Errorf("expected 4 predicates, got %d", len(predicates))
	}

	// Verify predicate types
	expectedTypes := map[string]bool{
		PredicateTypeSPDX:           false,
		PredicateTypeImageConfig:    false,
		PredicateTypeSLSAProvenance: false,
		PredicateTypeVMMetadata:     false,
	}

	for _, pred := range predicates {
		if _, ok := expectedTypes[pred.Type]; !ok {
			t.Errorf("unexpected predicate type: %s", pred.Type)
		}
		expectedTypes[pred.Type] = true
	}

	for predType, found := range expectedTypes {
		if !found {
			t.Errorf("missing expected predicate type: %s", predType)
		}
	}
}

// TestGenerateSBOMPredicate tests SBOM predicate generation from SPDX JSON files
// and validates the predicate type and data structure.
func TestGenerateSBOMPredicate(t *testing.T) {
	// Create temporary SBOM file
	tmpDir := t.TempDir()
	sbomPath := filepath.Join(tmpDir, "test.spdx.json")

	sbomData := map[string]interface{}{
		"spdxVersion": "SPDX-2.3",
		"name":        "test-package",
		"packages":    []string{"pkg1", "pkg2"},
	}
	sbomJSON, err := json.Marshal(sbomData)
	if err != nil {
		t.Fatalf("failed to marshal SBOM: %v", err)
	}
	if err := os.WriteFile(sbomPath, sbomJSON, 0644); err != nil {
		t.Fatalf("failed to write SBOM: %v", err)
	}

	// Generate predicate
	pred, err := generateSBOMPredicate(sbomPath)
	if err != nil {
		t.Fatalf("generateSBOMPredicate failed: %v", err)
	}

	// Verify predicate type
	if pred.Type != PredicateTypeSPDX {
		t.Errorf("expected type %s, got %s", PredicateTypeSPDX, pred.Type)
	}

	// Verify predicate data can be marshaled
	_, err = json.Marshal(pred.Data)
	if err != nil {
		t.Errorf("failed to marshal predicate data: %v", err)
	}
}

// TestGenerateVMMetadataPredicate tests VM metadata predicate generation including
// architecture, disk formats, cloud platforms, secure boot support, and SBOM availability.
func TestGenerateVMMetadataPredicate(t *testing.T) {
	artifacts := &Artifacts{
		Arch:            types.ParseArchitecture("aarch64"),
		Platform:        "gcp",
		DiskRaw:         []string{"/fake/disk.raw"},
		DiskQcow2:       []string{"/fake/disk.qcow2"},
		SecureBootFiles: []string{"/fake/secureboot.fd"},
		ApkoSBOMs:       []string{"/fake/sbom.json"},
		SyftSBOM:        "/fake/syft.json",
	}

	pred, err := generateVMMetadataPredicate(artifacts)
	if err != nil {
		t.Fatalf("generateVMMetadataPredicate() unexpected error: %v", err)
	}

	// Verify predicate type
	if pred.Type != PredicateTypeVMMetadata {
		t.Errorf("expected type %s, got %s", PredicateTypeVMMetadata, pred.Type)
	}

	// Verify predicate contains expected fields
	metadata, ok := pred.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("predicate data is not a map")
	}

	expectedFields := []string{"architecture", "diskFormats", "cloudPlatforms", "secureBootSupport", "sbomAvailable", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := metadata[field]; !exists {
			t.Errorf("missing expected field: %s", field)
		}
	}

	// Verify architecture
	if arch, ok := metadata["architecture"].(string); !ok || arch != "aarch64" {
		t.Errorf("expected architecture 'aarch64', got %v", metadata["architecture"])
	}

	// Verify disk formats
	diskFormats, ok := metadata["diskFormats"].([]string)
	if !ok {
		t.Errorf("diskFormats is not a string slice")
	} else if len(diskFormats) != 2 {
		t.Errorf("expected 2 disk formats, got %d", len(diskFormats))
	}

	// Verify secure boot support
	if secureBootSupport, ok := metadata["secureBootSupport"].(bool); !ok || !secureBootSupport {
		t.Errorf("expected secureBootSupport to be true")
	}

	// Verify cloud platforms (should be "gcp" based on artifacts.Platform)
	cloudPlatforms, ok := metadata["cloudPlatforms"].([]string)
	if !ok {
		t.Errorf("cloudPlatforms is not a string slice")
	} else {
		// Should include only: gcp (based on Platform field)
		expectedPlatforms := map[string]bool{"gcp": true}
		for _, platform := range cloudPlatforms {
			if !expectedPlatforms[platform] {
				t.Errorf("unexpected cloud platform: %s", platform)
			}
			delete(expectedPlatforms, platform)
		}
		if len(expectedPlatforms) > 0 {
			t.Errorf("missing expected cloud platforms: %v", expectedPlatforms)
		}
	}
}

// TestGenerateVMMetadataPredicateWithOVA tests VM metadata predicate generation
// specifically for VMware OVA format and validates correct format detection.
func TestGenerateVMMetadataPredicateWithOVA(t *testing.T) {
	artifacts := &Artifacts{
		Arch:            types.ParseArchitecture("x86_64"),
		Platform:        "vmware",
		DiskOva:         []string{"/fake/disk.ova"},
		SecureBootFiles: []string{"/fake/secureboot.fd"},
		ApkoSBOMs:       []string{"/fake/sbom.json"},
	}

	pred, err := generateVMMetadataPredicate(artifacts)
	if err != nil {
		t.Fatalf("generateVMMetadataPredicate() unexpected error: %v", err)
	}

	// Verify predicate type
	if pred.Type != PredicateTypeVMMetadata {
		t.Errorf("expected type %s, got %s", PredicateTypeVMMetadata, pred.Type)
	}

	// Verify predicate contains expected fields
	metadata, ok := pred.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("predicate data is not a map")
	}

	// Verify disk formats include ova
	diskFormats, ok := metadata["diskFormats"].([]string)
	if !ok {
		t.Errorf("diskFormats is not a string slice")
	} else if len(diskFormats) != 1 {
		t.Errorf("expected 1 disk format, got %d", len(diskFormats))
	} else if diskFormats[0] != "ova" {
		t.Errorf("expected disk format 'ova', got %s", diskFormats[0])
	}

	// Verify cloud platforms include vmware (based on Platform field)
	cloudPlatforms, ok := metadata["cloudPlatforms"].([]string)
	if !ok {
		t.Errorf("cloudPlatforms is not a string slice")
	} else {
		// Should include only: vmware (based on Platform field)
		expectedPlatforms := map[string]bool{"vmware": true}
		for _, platform := range cloudPlatforms {
			if !expectedPlatforms[platform] {
				t.Errorf("unexpected cloud platform: %s", platform)
			}
			delete(expectedPlatforms, platform)
		}
		if len(expectedPlatforms) > 0 {
			t.Errorf("missing expected cloud platforms: %v", expectedPlatforms)
		}
	}
}

// TestGenerateVMMetadataPredicateQEMU tests VM metadata predicate for QEMU
// platform and validates correct platform mapping to "qemu".
func TestGenerateVMMetadataPredicateQEMU(t *testing.T) {
	artifacts := &Artifacts{
		Arch:      types.ParseArchitecture("x86_64"),
		Platform:  "qemu",
		DiskRaw:   []string{"/fake/disk.raw"},
		DiskQcow2: []string{"/fake/disk.qcow2"},
		ApkoSBOMs: []string{"/fake/sbom.json"},
	}

	pred, err := generateVMMetadataPredicate(artifacts)
	if err != nil {
		t.Fatalf("generateVMMetadataPredicate() unexpected error: %v", err)
	}

	// Verify predicate type
	if pred.Type != PredicateTypeVMMetadata {
		t.Errorf("expected type %s, got %s", PredicateTypeVMMetadata, pred.Type)
	}

	// Verify predicate contains expected fields
	metadata, ok := pred.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("predicate data is not a map")
	}

	// Verify cloud platforms for qemu (should be qemu only)
	cloudPlatforms, ok := metadata["cloudPlatforms"].([]string)
	if !ok {
		t.Errorf("cloudPlatforms is not a string slice")
	} else {
		// Should include: qemu only
		expectedPlatforms := map[string]bool{"qemu": true}
		for _, platform := range cloudPlatforms {
			if !expectedPlatforms[platform] {
				t.Errorf("unexpected cloud platform: %s", platform)
			}
			delete(expectedPlatforms, platform)
		}
		if len(expectedPlatforms) > 0 {
			t.Errorf("missing expected cloud platforms: %v", expectedPlatforms)
		}
	}
}

// TestGenerateSLSAProvenancePredicate tests SLSA v1 provenance predicate generation
// and validates required SLSA fields (buildDefinition, runDetails, builder info).
func TestGenerateSLSAProvenancePredicate(t *testing.T) {
	artifacts := &Artifacts{
		Arch:      types.ParseArchitecture("x86_64"),
		Platform:  "aws",
		DiskRaw:   []string{"/fake/disk.raw"},
		DiskQcow2: []string{"/fake/disk.qcow2"},
	}

	pred, err := generateSLSAProvenancePredicate(artifacts)
	if err != nil {
		t.Fatalf("generateSLSAProvenancePredicate failed: %v", err)
	}

	// Verify predicate type
	if pred.Type != PredicateTypeSLSAProvenance {
		t.Errorf("expected type %s, got %s", PredicateTypeSLSAProvenance, pred.Type)
	}

	// Verify predicate structure
	provenance, ok := pred.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("predicate data is not a map")
	}

	// Check for required SLSA v1 fields
	if _, exists := provenance["buildDefinition"]; !exists {
		t.Error("missing buildDefinition field")
	}
	if _, exists := provenance["runDetails"]; !exists {
		t.Error("missing runDetails field")
	}

	// Verify buildDefinition structure
	buildDef, ok := provenance["buildDefinition"].(map[string]interface{})
	if !ok {
		t.Fatalf("buildDefinition is not a map")
	}
	if _, exists := buildDef["buildType"]; !exists {
		t.Error("missing buildType in buildDefinition")
	}

	// Verify runDetails structure
	runDetails, ok := provenance["runDetails"].(map[string]interface{})
	if !ok {
		t.Fatalf("runDetails is not a map")
	}
	if _, exists := runDetails["builder"]; !exists {
		t.Error("missing builder in runDetails")
	}
}

// TestGenerateImageConfigPredicate tests image configuration predicate generation from
// build.yaml files with various configs (packages, users, repositories).
func TestGenerateImageConfigPredicate(t *testing.T) {
	tests := []struct {
		name       string
		configData string
		arch       types.Architecture
		wantErr    bool
	}{
		{
			name: "valid config with packages",
			configData: `contents:
  packages:
    - wolfi-base
    - busybox
    - curl
accounts:
  users:
    - username: nonroot
      uid: 65532
`,
			arch:    types.ParseArchitecture("x86_64"),
			wantErr: false,
		},
		{
			name: "minimal valid config",
			configData: `contents:
  packages:
    - wolfi-base
`,
			arch:    types.ParseArchitecture("aarch64"),
			wantErr: false,
		},
		{
			name: "config with repositories",
			configData: `contents:
  repositories:
    - https://packages.wolfi.dev/os
  packages:
    - wolfi-base
`,
			arch:    types.ParseArchitecture("x86_64"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "build.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configData), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			// Generate predicate
			pred, err := generateImageConfigPredicate(configPath, tt.arch)

			if (err != nil) != tt.wantErr {
				t.Errorf("generateImageConfigPredicate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify predicate type
				if pred.Type != PredicateTypeImageConfig {
					t.Errorf("expected type %s, got %s", PredicateTypeImageConfig, pred.Type)
				}

				// Verify predicate structure
				predicateData, ok := pred.Data.(map[string]interface{})
				if !ok {
					t.Fatalf("predicate data is not a map")
				}

				// Verify architecture field
				arch, ok := predicateData["architecture"].(string)
				if !ok {
					t.Error("architecture field is missing or not a string")
				} else if arch != tt.arch.ToAPK() {
					t.Errorf("expected architecture %s, got %s", tt.arch.ToAPK(), arch)
				}

				// Verify configPath field
				cfgPath, ok := predicateData["configPath"].(string)
				if !ok {
					t.Error("configPath field is missing or not a string")
				} else if cfgPath != configPath {
					t.Errorf("expected configPath %s, got %s", configPath, cfgPath)
				}

				// Verify imageConfiguration field exists
				imgConfig, ok := predicateData["imageConfiguration"]
				if !ok {
					t.Error("imageConfiguration field is missing")
				} else {
					// Verify it's a types.ImageConfiguration struct
					_, ok := imgConfig.(types.ImageConfiguration)
					if !ok {
						t.Errorf("imageConfiguration is not types.ImageConfiguration, got %T", imgConfig)
					}
				}
			}
		})
	}
}

// TestGenerateImageConfigPredicateErrors tests error handling for missing or
// invalid config files during image config predicate generation.
func TestGenerateImageConfigPredicateErrors(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(t *testing.T) string
		arch       types.Architecture
		wantErr    bool
		errContains string
	}{
		{
			name: "missing config file",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/config/build.yaml"
			},
			arch:        types.ParseArchitecture("x86_64"),
			wantErr:     true,
			errContains: "reading build config",
		},
		{
			name: "invalid YAML syntax",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				configPath := filepath.Join(tmpDir, "build.yaml")
				invalidYAML := `contents:
  packages:
    - wolfi-base
  bad indent here
    - another-package
`
				if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
					t.Fatalf("failed to write test config: %v", err)
				}
				return configPath
			},
			arch:        types.ParseArchitecture("x86_64"),
			wantErr:     true,
			errContains: "parsing build config YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := tt.setupFunc(t)

			_, err := generateImageConfigPredicate(configPath, tt.arch)

			if (err != nil) != tt.wantErr {
				t.Errorf("generateImageConfigPredicate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGenerateSBOMPredicateErrors tests error handling for nonexistent, invalid,
// or empty SBOM files during SBOM predicate generation.
func TestGenerateSBOMPredicateErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) string
		wantErr     bool
		errContains string
	}{
		{
			name: "nonexistent SBOM file",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path/to/sbom.spdx.json"
			},
			wantErr:     true,
			errContains: "reading SBOM file",
		},
		{
			name: "invalid JSON in SBOM file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				sbomPath := filepath.Join(tmpDir, "invalid.spdx.json")
				invalidJSON := `{"spdxVersion": "SPDX-2.3", "name": "incomplete`
				if err := os.WriteFile(sbomPath, []byte(invalidJSON), 0644); err != nil {
					t.Fatalf("failed to write test SBOM: %v", err)
				}
				return sbomPath
			},
			wantErr:     true,
			errContains: "parsing SBOM JSON",
		},
		{
			name: "empty SBOM file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				sbomPath := filepath.Join(tmpDir, "empty.spdx.json")
				if err := os.WriteFile(sbomPath, []byte(""), 0644); err != nil {
					t.Fatalf("failed to write test SBOM: %v", err)
				}
				return sbomPath
			},
			wantErr:     true,
			errContains: "parsing SBOM JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sbomPath := tt.setupFunc(t)

			_, err := generateSBOMPredicate(sbomPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("generateSBOMPredicate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestGeneratePredicatesErrors tests overall error propagation during predicate generation
// when SBOM or config predicates fail.
func TestGeneratePredicatesErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) *Artifacts
		wantErr     bool
		errContains string
	}{
		{
			name: "invalid SBOM file",
			setupFunc: func(t *testing.T) *Artifacts {
				return &Artifacts{
					Arch:      types.ParseArchitecture("x86_64"),
					Platform:  PlatformAWS,
					ApkoSBOMs: []string{"/nonexistent/sbom.spdx.json"},
					DiskRaw:   []string{"disk.raw"},
				}
			},
			wantErr:     true,
			errContains: "generating SBOM predicate",
		},
		{
			name: "invalid config file",
			setupFunc: func(t *testing.T) *Artifacts {
				return &Artifacts{
					Arch:            types.ParseArchitecture("x86_64"),
					Platform:        PlatformAWS,
					BuildConfigPath: "/nonexistent/build.yaml",
					DiskRaw:         []string{"disk.raw"},
				}
			},
			wantErr:     true,
			errContains: "generating image config predicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := tt.setupFunc(t)

			_, err := generatePredicates(artifacts)

			if (err != nil) != tt.wantErr {
				t.Errorf("generatePredicates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestGeneratePredicatesEdgeCases tests predicate generation with minimal artifacts
// (no SBOMs, no config) and fully populated artifacts scenarios.
func TestGeneratePredicatesEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		artifacts      *Artifacts
		wantPredicates int
		checkTypes     []string
	}{
		{
			name: "minimal artifacts - no SBOMs, no config",
			artifacts: &Artifacts{
				Arch:     types.ParseArchitecture("x86_64"),
				Platform: PlatformQEMU,
				DiskRaw:  []string{"disk.raw"},
			},
			wantPredicates: 2, // SLSA + VM metadata
			checkTypes: []string{
				PredicateTypeSLSAProvenance,
				PredicateTypeVMMetadata,
			},
		},
		{
			name: "all fields populated",
			artifacts: func() *Artifacts {
				tmpDir := t.TempDir()

				// Create SBOM file
				sbomPath := filepath.Join(tmpDir, "sbom.spdx.json")
				sbomData := map[string]interface{}{
					"spdxVersion": "SPDX-2.3",
					"name":        "test-sbom",
				}
				sbomJSON, _ := json.Marshal(sbomData)
				os.WriteFile(sbomPath, sbomJSON, 0644)

				// Create config file
				configPath := filepath.Join(tmpDir, "build.yaml")
				configData := `contents:
  packages:
    - wolfi-base
`
				os.WriteFile(configPath, []byte(configData), 0644)

				return &Artifacts{
					Arch:            types.ParseArchitecture("x86_64"),
					Platform:        PlatformAWS,
					ApkoSBOMs:       []string{sbomPath},
					BuildConfigPath: configPath,
					DiskVmdk:        []string{"disk.vmdk"},
				}
			}(),
			wantPredicates: 4, // SBOM + Config + SLSA + VM metadata
			checkTypes: []string{
				PredicateTypeSPDX,
				PredicateTypeImageConfig,
				PredicateTypeSLSAProvenance,
				PredicateTypeVMMetadata,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicates, err := generatePredicates(tt.artifacts)
			if err != nil {
				t.Fatalf("generatePredicates() unexpected error: %v", err)
			}

			if len(predicates) != tt.wantPredicates {
				t.Errorf("expected %d predicates, got %d", tt.wantPredicates, len(predicates))
			}

			// Check that expected types are present
			foundTypes := make(map[string]bool)
			for _, pred := range predicates {
				foundTypes[pred.Type] = true
			}

			for _, expectedType := range tt.checkTypes {
				if !foundTypes[expectedType] {
					t.Errorf("missing expected predicate type: %s", expectedType)
				}
			}
		})
	}
}

// TestGenerateSLSAProvenancePredicateWithEnv tests SLSA provenance generation with
// environment variables (GITHUB_SHA, COMMIT, GITHUB_REPOSITORY) and validates proper
// commit hash extraction and builder ID construction.
func TestGenerateSLSAProvenancePredicateWithEnv(t *testing.T) {
	// Save original env vars
	origGitHubSHA := os.Getenv("GITHUB_SHA")
	origCommit := os.Getenv("COMMIT")
	origGitHubRepo := os.Getenv("GITHUB_REPOSITORY")

	// Restore after test
	defer func() {
		os.Setenv("GITHUB_SHA", origGitHubSHA)
		os.Setenv("COMMIT", origCommit)
		os.Setenv("GITHUB_REPOSITORY", origGitHubRepo)
	}()

	tests := []struct {
		name          string
		setupEnv      func()
		artifacts     *Artifacts
		wantCommit    string
		wantBuilderID string
	}{
		{
			name: "with GITHUB_SHA",
			setupEnv: func() {
				os.Setenv("GITHUB_SHA", "abc123def456")
				os.Unsetenv("COMMIT")
				os.Unsetenv("GITHUB_REPOSITORY")
			},
			artifacts: &Artifacts{
				Arch:     types.ParseArchitecture("x86_64"),
				Platform: PlatformAWS,
				DiskRaw:  []string{"disk.raw"},
			},
			wantCommit:    "abc123def456",
			wantBuilderID: "https://github.com/chainguard-dev/wolfi-vm",
		},
		{
			name: "with COMMIT fallback",
			setupEnv: func() {
				os.Unsetenv("GITHUB_SHA")
				os.Setenv("COMMIT", "fallback789")
				os.Unsetenv("GITHUB_REPOSITORY")
			},
			artifacts: &Artifacts{
				Arch:     types.ParseArchitecture("aarch64"),
				Platform: PlatformAzure,
				DiskVhd:  []string{"disk.vhd"},
			},
			wantCommit:    "fallback789",
			wantBuilderID: "https://github.com/chainguard-dev/wolfi-vm",
		},
		{
			name: "with GITHUB_REPOSITORY",
			setupEnv: func() {
				os.Setenv("GITHUB_SHA", "xyz789")
				os.Setenv("GITHUB_REPOSITORY", "myorg/myrepo")
			},
			artifacts: &Artifacts{
				Arch:     types.ParseArchitecture("x86_64"),
				Platform: PlatformGCP,
				DiskRaw:  []string{"disk.raw"},
			},
			wantCommit:    "xyz789",
			wantBuilderID: "https://github.com/myorg/myrepo",
		},
		{
			name: "no env vars - defaults",
			setupEnv: func() {
				os.Unsetenv("GITHUB_SHA")
				os.Unsetenv("COMMIT")
				os.Unsetenv("GITHUB_REPOSITORY")
			},
			artifacts: &Artifacts{
				Arch:     types.ParseArchitecture("x86_64"),
				Platform: PlatformQEMU,
				DiskQcow2: []string{"disk.qcow2"},
			},
			wantCommit:    "unknown",
			wantBuilderID: "https://github.com/chainguard-dev/wolfi-vm",
		},
		{
			name: "with BuildConfigPath",
			setupEnv: func() {
				os.Setenv("GITHUB_SHA", "config123")
				os.Unsetenv("GITHUB_REPOSITORY")
			},
			artifacts: &Artifacts{
				Arch:            types.ParseArchitecture("x86_64"),
				Platform:        PlatformAWS,
				BuildConfigPath: "configs/aws-base/build.yaml",
				DiskVmdk:        []string{"disk.vmdk"},
			},
			wantCommit:    "config123",
			wantBuilderID: "https://github.com/chainguard-dev/wolfi-vm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()

			pred, err := generateSLSAProvenancePredicate(tt.artifacts)
			if err != nil {
				t.Fatalf("generateSLSAProvenancePredicate() error: %v", err)
			}

			if pred.Type != PredicateTypeSLSAProvenance {
				t.Errorf("expected type %s, got %s", PredicateTypeSLSAProvenance, pred.Type)
			}

			// Verify structure
			provData, ok := pred.Data.(map[string]interface{})
			if !ok {
				t.Fatal("predicate data is not a map")
			}

			// Check runDetails
			runDetails, ok := provData["runDetails"].(map[string]interface{})
			if !ok {
				t.Fatal("runDetails is missing or not a map")
			}

			builder, ok := runDetails["builder"].(map[string]interface{})
			if !ok {
				t.Fatal("builder is missing or not a map")
			}

			// Check builder ID
			if builderID, ok := builder["id"].(string); !ok || builderID != tt.wantBuilderID {
				t.Errorf("builder ID = %v, want %s", builder["id"], tt.wantBuilderID)
			}

			// Check commit
			version, ok := builder["version"].(map[string]interface{})
			if !ok {
				t.Fatal("version is missing or not a map")
			}

			if commit, ok := version["commit"].(string); !ok || commit != tt.wantCommit {
				t.Errorf("commit = %v, want %s", version["commit"], tt.wantCommit)
			}

			// Check buildDefinition
			buildDef, ok := provData["buildDefinition"].(map[string]interface{})
			if !ok {
				t.Fatal("buildDefinition is missing or not a map")
			}

			// Check internalParameters has configPath if BuildConfigPath is set
			internalParams, ok := buildDef["internalParameters"].(map[string]interface{})
			if !ok {
				t.Fatal("internalParameters is missing or not a map")
			}

			if tt.artifacts.BuildConfigPath != "" {
				if configPath, ok := internalParams["configPath"].(string); !ok || configPath != tt.artifacts.BuildConfigPath {
					t.Errorf("configPath = %v, want %s", internalParams["configPath"], tt.artifacts.BuildConfigPath)
				}
			}
		})
	}
}
