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
	"fmt"
	"os"
	"path/filepath"

	"chainguard.dev/apko/pkg/build/types"
)

type Artifacts struct {
	Arch            types.Architecture
	Platform        Platform // Target platform extracted from config (e.g., PlatformAWS, PlatformAzure, PlatformGCP)
	ApkoTarball     string   // Path to apko*.tar.gz rootfs tarball
	DiskRaw         []string // Paths to .raw disk files
	DiskQcow2       []string // Paths to .qcow2 disk files
	DiskVmdk        []string // Paths to .vmdk disk files (e.g., disk.vmdk, disk-flat.vmdk)
	DiskVhd         []string // Paths to .vhd/.vpc disk files
	DiskOva         []string // Paths to .ova disk files (VMware OVA format)
	ApkoSBOMs       []string // Paths to apko-generated sbom-*.spdx.json files
	SyftSBOM        string   // Path to syft.sbom.json
	SecureBootFiles []string // Paths to UEFI/secure boot files (*.auth, *.esl, *.fd, *.bin, uefi-*.json)
	BuildConfigPath string   // Path to original build.yaml configuration
}

// Summary returns a list of artifact descriptions for logging
func (a *Artifacts) Summary() []string {
	var summary []string

	// Summarize disk and file artifacts
	artifactTypes := []struct {
		files    []string
		singular string
		plural   string
	}{
		{a.DiskRaw, "disk.raw.v1", ".raw files"},
		{a.DiskQcow2, "disk.qcow2.v1", ".qcow2 files"},
		{a.DiskVmdk, "disk.vmdk.v1", ".vmdk files"},
		{a.DiskVhd, "disk.vhd.v1", ".vhd files"},
		{a.DiskOva, "disk.ova.v1", ".ova files"},
		{a.SecureBootFiles, "secure boot file", "secure boot files"},
	}

	for _, at := range artifactTypes {
		if len(at.files) > 0 {
			if len(at.files) == 1 {
				summary = append(summary, at.singular)
			} else {
				summary = append(summary, fmt.Sprintf("%d %s", len(at.files), at.plural))
			}
		}
	}

	// Add SBOMs
	if len(a.ApkoSBOMs) > 0 {
		if len(a.ApkoSBOMs) == 1 {
			summary = append(summary, "apko SBOM")
		} else {
			summary = append(summary, fmt.Sprintf("%d apko SBOMs", len(a.ApkoSBOMs)))
		}
	}
	if a.SyftSBOM != "" {
		summary = append(summary, "syft SBOM")
	}

	return summary
}

// LoadArtifactsFromDir loads artifacts from a standard output directory structure
func LoadArtifactsFromDir(outputDir string, arch types.Architecture, platform Platform) (*Artifacts, error) {
	artifacts := &Artifacts{
		Arch:     arch,
		Platform: platform,
	}

	// Derive build config path from output directory
	// Output structure: output/<arch>/<config-name>/ → configs/<config-name>/build.yaml
	configName := filepath.Base(outputDir)
	configPath := filepath.Join("configs", configName, "build.yaml")
	if _, err := os.Stat(configPath); err == nil {
		artifacts.BuildConfigPath = configPath
	}

	// Check for syft SBOM
	sbomPath := filepath.Join(outputDir, "syft.sbom.json")
	if _, err := os.Stat(sbomPath); err == nil {
		artifacts.SyftSBOM = sbomPath
	}

	// Scan directory and categorize all files
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, fmt.Errorf("reading output directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		fullPath := filepath.Join(outputDir, filename)

		// Check for apko tarball
		if matched, _ := filepath.Match("apko*.tar.gz", filename); matched {
			artifacts.ApkoTarball = fullPath
			continue
		}

		// Check for apko SBOM (sbom-*.spdx.json)
		if matched, _ := filepath.Match("sbom-*.spdx.json", filename); matched {
			artifacts.ApkoSBOMs = append(artifacts.ApkoSBOMs, fullPath)
			continue
		}

		// Skip files that are handled separately or temporary
		skipPatterns := []string{
			"syft.sbom.json", // Syft SBOM already handled above
			"*.tmp.*",        // Temporary files from Makefile
		}

		skip := false
		for _, pattern := range skipPatterns {
			if matched, _ := filepath.Match(pattern, filename); matched {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Categorize by file extension
		extToSlice := map[string]*[]string{
			".raw":   &artifacts.DiskRaw,
			".qcow2": &artifacts.DiskQcow2,
			".vmdk":  &artifacts.DiskVmdk,
			".vhd":   &artifacts.DiskVhd,
			".vpc":   &artifacts.DiskVhd,
			".ova":   &artifacts.DiskOva,
		}

		ext := filepath.Ext(filename)
		if targetSlice, ok := extToSlice[ext]; ok {
			*targetSlice = append(*targetSlice, fullPath)
		} else if isSecureBootFile(filename) {
			artifacts.SecureBootFiles = append(artifacts.SecureBootFiles, fullPath)
		} else {
			// Unknown file type - log but don't publish
			fmt.Fprintf(os.Stderr, "WARNING: Unknown file in output directory (not published): %s\n", filename)
		}
	}

	// At least one disk artifact must exist
	if len(artifacts.DiskRaw) == 0 && len(artifacts.DiskQcow2) == 0 &&
		len(artifacts.DiskVmdk) == 0 && len(artifacts.DiskVhd) == 0 && len(artifacts.DiskOva) == 0 {
		return nil, fmt.Errorf("no disk artifacts found in %s (expected .raw, .qcow2, .vmdk, .vhd, or .ova files)", outputDir)
	}

	return artifacts, nil
}

// Validate ensures all required artifact files exist
func (a *Artifacts) Validate() error {
	// Build list of all files to validate
	fileListsToValidate := []struct {
		files []string
		name  string
	}{
		{a.DiskRaw, "disk"},
		{a.DiskQcow2, "disk"},
		{a.DiskVmdk, "disk"},
		{a.DiskVhd, "disk"},
		{a.DiskOva, "disk"},
		{a.SecureBootFiles, "secure boot"},
	}

	// Add SBOMs if present
	if len(a.ApkoSBOMs) > 0 {
		fileListsToValidate = append(fileListsToValidate, struct {
			files []string
			name  string
		}{a.ApkoSBOMs, "apko SBOM"})
	}
	if a.SyftSBOM != "" {
		fileListsToValidate = append(fileListsToValidate, struct {
			files []string
			name  string
		}{[]string{a.SyftSBOM}, "syft SBOM"})
	}

	// Validate all files
	for _, fv := range fileListsToValidate {
		for _, path := range fv.files {
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("%s file not found at %s: %w", fv.name, path, err)
			}
		}
	}

	return nil
}
