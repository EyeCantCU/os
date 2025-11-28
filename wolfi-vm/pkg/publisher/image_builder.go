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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chainguard.dev/apko/pkg/build/oci"
	"chainguard.dev/apko/pkg/build/types"
	"github.com/chainguard-dev/clog"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

// createImages creates separate OCI images for each artifact type (apko rootfs, disk formats, sbom, etc.)
func (p *Publisher) createImages(ctx context.Context, artifacts *Artifacts) ([]ImageWithMetadata, error) {
	log := clog.FromContext(ctx)
	var images []ImageWithMetadata

	// Create base ImageConfiguration for common annotations
	baseIC := types.ImageConfiguration{
		Annotations: make(map[string]string),
	}
	baseIC.Annotations[AnnotationBuildTime] = time.Now().Format(time.RFC3339)
	baseIC.Annotations[AnnotationArchitecture] = artifacts.Arch.ToAPK()

	// Create apko rootfs image if present
	apkoLayer, err := p.loadApkoTarball(artifacts.ApkoTarball)
	if err != nil {
		return nil, fmt.Errorf("loading apko tarball: %w", err)
	}
	if apkoLayer != nil {
		ic := copyICWithArtifactType(baseIC, "apko.v1")

		img, err := oci.BuildImageFromLayers(ctx, empty.Image, []v1.Layer{apkoLayer}, ic, time.Now(), artifacts.Arch)
		if err != nil {
			return nil, fmt.Errorf("building apko image: %w", err)
		}
		images = append(images, ImageWithMetadata{
			Image:        img,
			Arch:         artifacts.Arch,
			ArtifactType: "apko.v1",
		})
		log.Infof("Created apko rootfs image")
	}

	// Create images for each disk format
	diskFormats := []struct {
		files        []string
		mediaType    ggcrtypes.MediaType
		artifactType string
		name         string
	}{
		{artifacts.DiskRaw, MediaTypeVMDiskRaw, "disk.raw.tgz.v1", "disk.raw.tgz"},
		{artifacts.DiskQcow2, MediaTypeVMDiskQcow2, "disk.qcow2.v1", "disk.qcow2"},
		{artifacts.DiskVmdk, MediaTypeVMDiskVmdk, "disk.vmdk.v1", "disk.vmdk"},
		{artifacts.DiskVhd, MediaTypeVMDiskVhd, "disk.vhd.v1", "disk.vhd"},
		{artifacts.DiskOva, MediaTypeVMDiskOva, "disk.ova.v1", "disk.ova"},
	}

	for _, df := range diskFormats {
		if len(df.files) > 0 {
			layer, err := p.createMultiFileLayer(df.files, df.mediaType)
			if err != nil {
				return nil, fmt.Errorf("creating %s layer: %w", df.name, err)
			}

			// Create annotations for artifact
			annotations := make(map[string]string)
			for k, v := range baseIC.Annotations {
				annotations[k] = v
			}
			annotations[AnnotationArtifactType] = df.artifactType

			// Build as OCI artifact (not container image) with empty config
			img, err := buildArtifactImage(layer, annotations, time.Now())
			if err != nil {
				return nil, fmt.Errorf("building %s artifact: %w", df.name, err)
			}
			images = append(images, ImageWithMetadata{
				Image:        img,
				Arch:         artifacts.Arch,
				ArtifactType: df.artifactType,
			})
			log.Infof("Created %s artifact image with %d files", df.name, len(df.files))
		}
	}

	// Helper to create an artifact image from file artifacts
	createFileArtifactImage := func(files []string, mediaType ggcrtypes.MediaType, artifactType, name string) error {
		if len(files) == 0 {
			return nil
		}

		layer, err := p.createMultiFileLayer(files, mediaType)
		if err != nil {
			return fmt.Errorf("creating %s layer: %w", name, err)
		}

		// Create annotations for artifact
		annotations := make(map[string]string)
		for k, v := range baseIC.Annotations {
			annotations[k] = v
		}
		annotations[AnnotationArtifactType] = artifactType

		// Build as OCI artifact (not container image) with empty config
		img, err := buildArtifactImage(layer, annotations, time.Now())
		if err != nil {
			return fmt.Errorf("building %s artifact: %w", name, err)
		}

		images = append(images, ImageWithMetadata{
			Image:        img,
			Arch:         artifacts.Arch,
			ArtifactType: artifactType,
		})
		log.Infof("Created %s artifact image with %d files", name, len(files))
		return nil
	}

	// Create SBOM and secure boot artifact images
	if err := createFileArtifactImage(artifacts.ApkoSBOMs, MediaTypeVMApkoSbom, "apko-sbom.v1", "apko SBOM"); err != nil {
		return nil, err
	}

	var syftFiles []string
	if artifacts.SyftSBOM != "" {
		syftFiles = []string{artifacts.SyftSBOM}
	}
	if err := createFileArtifactImage(syftFiles, MediaTypeVMSyftSbom, "syft-sbom.v1", "syft SBOM"); err != nil {
		return nil, err
	}

	if err := createFileArtifactImage(artifacts.SecureBootFiles, MediaTypeVMSecureBoot, "secureboot.v1", "secure boot"); err != nil {
		return nil, err
	}

	return images, nil
}

// artifactImage wraps a v1.Image to present it as an OCI artifact with empty config.
// Per OCI spec, artifacts should have config = {} and config media type = application/vnd.oci.empty.v1+json
type artifactImage struct {
	base           v1.Image
	cachedManifest *v1.Manifest
	cachedRaw      []byte
	cachedDigest   v1.Hash
	cachedSize     int64
	configHash     v1.Hash // sha256 of "{}"
}

func (a *artifactImage) Layers() ([]v1.Layer, error)                    { return a.base.Layers() }
func (a *artifactImage) MediaType() (ggcrtypes.MediaType, error)        { return a.base.MediaType() }
func (a *artifactImage) Size() (int64, error)                           { return a.cachedSize, nil }
func (a *artifactImage) ConfigName() (v1.Hash, error)                   { return a.configHash, nil }
func (a *artifactImage) LayerByDigest(h v1.Hash) (v1.Layer, error)      { return a.base.LayerByDigest(h) }
func (a *artifactImage) LayerByDiffID(h v1.Hash) (v1.Layer, error)      { return a.base.LayerByDiffID(h) }

// RawConfigFile returns empty JSON object per OCI artifact spec
func (a *artifactImage) RawConfigFile() ([]byte, error) {
	return []byte("{}"), nil
}

// ConfigFile returns nil since artifacts don't have standard container configs
func (a *artifactImage) ConfigFile() (*v1.ConfigFile, error) {
	return nil, fmt.Errorf("artifacts do not have ConfigFile, only empty config blob")
}

// Digest returns the digest of the artifact manifest
func (a *artifactImage) Digest() (v1.Hash, error) {
	return a.cachedDigest, nil
}

// Manifest returns the artifact manifest with modified config descriptor
func (a *artifactImage) Manifest() (*v1.Manifest, error) {
	return a.cachedManifest, nil
}

// RawManifest returns the serialized manifest
func (a *artifactImage) RawManifest() ([]byte, error) {
	return a.cachedRaw, nil
}

// buildArtifactImage creates an OCI artifact image (not a container image) with empty config.
// Per OCI spec: https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidelines-for-artifact-usage
// Artifacts use:
//   - Empty config blob: {}
//   - Config media type: application/vnd.oci.empty.v1+json
//   - Artifact type annotation to identify the content
func buildArtifactImage(layer v1.Layer, annotations map[string]string, created time.Time) (v1.Image, error) {
	// Start with empty image and append our layer with history
	adds := []mutate.Addendum{
		{
			Layer: layer,
			History: v1.History{
				Author:    "wolfi-vm",
				Comment:   "VM artifact layer",
				CreatedBy: "wolfi-vm publisher",
				Created:   v1.Time{Time: created},
			},
		},
	}

	// Build base image with layer
	baseImage := empty.Image
	baseImage = mutate.MediaType(baseImage, ggcrtypes.OCIManifestSchema1)

	img, err := mutate.Append(baseImage, adds...)
	if err != nil {
		return nil, fmt.Errorf("appending layer to empty image: %w", err)
	}

	// Add annotations
	img = mutate.Annotations(img, annotations).(v1.Image)

	// Get base manifest to modify
	baseManifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("getting base manifest: %w", err)
	}

	// Create modified manifest with empty config
	manifest := baseManifest.DeepCopy()

	// Calculate digest of empty config
	emptyConfig := []byte("{}")
	configHash, _, err := v1.SHA256(bytes.NewReader(emptyConfig))
	if err != nil {
		return nil, fmt.Errorf("calculating empty config digest: %w", err)
	}

	// Update config descriptor to use empty media type
	manifest.Config.MediaType = MediaTypeOCIEmpty
	manifest.Config.Size = 2 // size of "{}"
	manifest.Config.Digest = configHash

	// Serialize manifest to compute digest
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}

	// Compute manifest digest
	manifestDigest, _, err := v1.SHA256(bytes.NewReader(rawManifest))
	if err != nil {
		return nil, fmt.Errorf("calculating manifest digest: %w", err)
	}

	// Wrap in artifact image with cached values
	return &artifactImage{
		base:           img,
		cachedManifest: manifest,
		cachedRaw:      rawManifest,
		cachedDigest:   manifestDigest,
		cachedSize:     int64(len(rawManifest)),
		configHash:     configHash,
	}, nil
}
