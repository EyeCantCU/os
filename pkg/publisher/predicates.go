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
	"fmt"
	"os"
	"time"

	"chainguard.dev/apko/pkg/build/types"
	"gopkg.in/yaml.v3"
)

const (
	// Predicate types following standard specifications
	PredicateTypeSPDX             = "https://spdx.dev/Document"
	PredicateTypeSLSAProvenance   = "https://slsa.dev/provenance/v1"
	PredicateTypeImageConfig      = "https://chainguard.dev/image-configuration"
	PredicateTypeVMMetadata       = "https://chainguard.dev/vm-metadata"
)

// Predicate represents an attestation predicate with its type and content
type Predicate struct {
	Type string      // Predicate type URI
	Data interface{} // Predicate data (will be JSON marshaled)
}

// generatePredicates creates all attestation predicates for the given artifacts
func generatePredicates(artifacts *Artifacts) ([]Predicate, error) {
	var predicates []Predicate

	// 1. SBOM Predicate (SPDX) - from apko SBOMs
	if len(artifacts.ApkoSBOMs) > 0 {
		// Use the first SBOM (typically the main one)
		sbomPred, err := generateSBOMPredicate(artifacts.ApkoSBOMs[0])
		if err != nil {
			return nil, fmt.Errorf("generating SBOM predicate: %w", err)
		}
		predicates = append(predicates, sbomPred)
	}

	// 2. Build Configuration Predicate
	if artifacts.BuildConfigPath != "" {
		configPred, err := generateImageConfigPredicate(artifacts.BuildConfigPath, artifacts.Arch)
		if err != nil {
			return nil, fmt.Errorf("generating image config predicate: %w", err)
		}
		predicates = append(predicates, configPred)
	}

	// 3. SLSA Provenance Predicate
	provPred, err := generateSLSAProvenancePredicate(artifacts)
	if err != nil {
		return nil, fmt.Errorf("generating SLSA provenance predicate: %w", err)
	}
	predicates = append(predicates, provPred)

	// 4. VM-Specific Metadata Predicate
	vmMetaPred, err := generateVMMetadataPredicate(artifacts)
	if err != nil {
		return nil, fmt.Errorf("generating VM metadata predicate: %w", err)
	}
	predicates = append(predicates, vmMetaPred)

	return predicates, nil
}

// generateSBOMPredicate creates an SBOM predicate from an SPDX JSON file
func generateSBOMPredicate(sbomPath string) (Predicate, error) {
	sbomData, err := os.ReadFile(sbomPath)
	if err != nil {
		return Predicate{}, fmt.Errorf("reading SBOM file %s: %w", sbomPath, err)
	}

	// Parse SBOM JSON to ensure it's valid
	var sbomJSON map[string]interface{}
	if err := json.Unmarshal(sbomData, &sbomJSON); err != nil {
		return Predicate{}, fmt.Errorf("parsing SBOM JSON: %w", err)
	}

	return Predicate{
		Type: PredicateTypeSPDX,
		Data: sbomJSON,
	}, nil
}

// generateImageConfigPredicate creates a predicate from the build configuration
func generateImageConfigPredicate(configPath string, arch types.Architecture) (Predicate, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return Predicate{}, fmt.Errorf("reading build config %s: %w", configPath, err)
	}

	// Parse YAML config
	var config types.ImageConfiguration
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return Predicate{}, fmt.Errorf("parsing build config YAML: %w", err)
	}

	// Create predicate with config data
	predicateData := map[string]interface{}{
		"imageConfiguration": config,
		"architecture":       arch.ToAPK(),
		"configPath":         configPath,
	}

	return Predicate{
		Type: PredicateTypeImageConfig,
		Data: predicateData,
	}, nil
}

// generateSLSAProvenancePredicate creates a SLSA v1 provenance predicate
func generateSLSAProvenancePredicate(artifacts *Artifacts) (Predicate, error) {
	// Determine git commit if available
	gitCommit := os.Getenv("GITHUB_SHA")
	if gitCommit == "" {
		gitCommit = os.Getenv("COMMIT")
	}
	if gitCommit == "" {
		gitCommit = "unknown"
	}

	// Determine builder context
	builderID := "https://github.com/chainguard-dev/wolfi-vm"
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		builderID = fmt.Sprintf("https://github.com/%s", repo)
	}

	// Build parameters from artifacts
	buildParams := map[string]interface{}{
		"architecture": artifacts.Arch.ToAPK(),
	}
	if artifacts.BuildConfigPath != "" {
		buildParams["configPath"] = artifacts.BuildConfigPath
	}

	// Collect disk formats
	diskFormatTypes := CollectDiskFormats(artifacts)
	diskFormats := make([]string, len(diskFormatTypes))
	for i, f := range diskFormatTypes {
		diskFormats[i] = string(f)
	}
	buildParams["diskFormats"] = diskFormats

	// SLSA v1 provenance structure
	provenance := map[string]interface{}{
		"buildDefinition": map[string]interface{}{
			"buildType": "https://chainguard.dev/slsa-build-type@v1",
			"externalParameters": map[string]interface{}{
				"architecture": artifacts.Arch.ToAPK(),
				"diskFormats":  diskFormats,
			},
			"internalParameters": buildParams,
		},
		"runDetails": map[string]interface{}{
			"builder": map[string]interface{}{
				"id": builderID,
				"version": map[string]interface{}{
					"commit": gitCommit,
				},
			},
			"metadata": map[string]interface{}{
				"invocationId": fmt.Sprintf("%s-%d", artifacts.Arch.ToAPK(), time.Now().Unix()),
				"startedOn":    time.Now().Format(time.RFC3339),
			},
		},
	}

	return Predicate{
		Type: PredicateTypeSLSAProvenance,
		Data: provenance,
	}, nil
}

// generateVMMetadataPredicate creates a VM-specific metadata predicate
func generateVMMetadataPredicate(artifacts *Artifacts) (Predicate, error) {
	// Collect available disk formats
	diskFormatTypes := CollectDiskFormats(artifacts)
	diskFormats := make([]string, len(diskFormatTypes))
	for i, f := range diskFormatTypes {
		diskFormats[i] = string(f)
	}

	// Determine cloud platform support based on target platform from config
	cloudPlatforms, err := artifacts.Platform.ToCloudPlatforms()
	if err != nil {
		return Predicate{}, fmt.Errorf("failed to get cloud platforms: %w", err)
	}

	metadata := map[string]interface{}{
		"architecture":    artifacts.Arch.ToAPK(),
		"diskFormats":     diskFormats,
		"cloudPlatforms":  cloudPlatforms,
		"secureBootSupport": len(artifacts.SecureBootFiles) > 0,
		"sbomAvailable": map[string]bool{
			"apko": len(artifacts.ApkoSBOMs) > 0,
			"syft": artifacts.SyftSBOM != "",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	return Predicate{
		Type: PredicateTypeVMMetadata,
		Data: metadata,
	}, nil
}
