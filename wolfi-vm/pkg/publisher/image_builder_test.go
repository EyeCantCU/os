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
	"io"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

// TestBuildArtifactImage verifies that artifacts are created with empty config per OCI spec
func TestBuildArtifactImage(t *testing.T) {
	// Create a simple layer with test content
	content := []byte("test artifact content")
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	})
	if err != nil {
		t.Fatalf("creating test layer: %v", err)
	}

	// Create artifact with annotations
	annotations := map[string]string{
		AnnotationBuildTime:    time.Now().Format(time.RFC3339),
		AnnotationArchitecture: "x86_64",
		AnnotationArtifactType: "disk.raw.v1",
	}

	img, err := buildArtifactImage(layer, annotations, time.Now())
	if err != nil {
		t.Fatalf("buildArtifactImage() failed: %v", err)
	}

	// Verify empty config blob
	t.Run("config is empty JSON", func(t *testing.T) {
		rawConfig, err := img.RawConfigFile()
		if err != nil {
			t.Fatalf("RawConfigFile() failed: %v", err)
		}

		expectedConfig := []byte("{}")
		if !bytes.Equal(rawConfig, expectedConfig) {
			t.Errorf("RawConfigFile() = %q, want %q", string(rawConfig), string(expectedConfig))
		}
	})

	// Verify config descriptor media type
	t.Run("config mediaType is application/vnd.oci.empty.v1+json", func(t *testing.T) {
		manifest, err := img.Manifest()
		if err != nil {
			t.Fatalf("Manifest() failed: %v", err)
		}

		if manifest.Config.MediaType != MediaTypeOCIEmpty {
			t.Errorf("Config.MediaType = %q, want %q", manifest.Config.MediaType, MediaTypeOCIEmpty)
		}
	})

	// Verify config descriptor size
	t.Run("config size is 2 bytes", func(t *testing.T) {
		manifest, err := img.Manifest()
		if err != nil {
			t.Fatalf("Manifest() failed: %v", err)
		}

		if manifest.Config.Size != 2 {
			t.Errorf("Config.Size = %d, want 2", manifest.Config.Size)
		}
	})

	// Verify config descriptor digest matches empty config
	t.Run("config digest matches empty config hash", func(t *testing.T) {
		manifest, err := img.Manifest()
		if err != nil {
			t.Fatalf("Manifest() failed: %v", err)
		}

		// Calculate expected digest of "{}"
		emptyConfig := []byte("{}")
		expectedHash, _, err := v1.SHA256(bytes.NewReader(emptyConfig))
		if err != nil {
			t.Fatalf("calculating expected hash: %v", err)
		}

		if manifest.Config.Digest != expectedHash {
			t.Errorf("Config.Digest = %v, want %v", manifest.Config.Digest, expectedHash)
		}
	})

	// Verify annotations are preserved
	t.Run("annotations are preserved", func(t *testing.T) {
		manifest, err := img.Manifest()
		if err != nil {
			t.Fatalf("Manifest() failed: %v", err)
		}

		for k, expectedValue := range annotations {
			actualValue, ok := manifest.Annotations[k]
			if !ok {
				t.Errorf("annotation %q missing from manifest", k)
				continue
			}
			if actualValue != expectedValue {
				t.Errorf("annotation %q = %q, want %q", k, actualValue, expectedValue)
			}
		}
	})

	// Verify manifest media type is OCI
	t.Run("manifest mediaType is OCI", func(t *testing.T) {
		mediaType, err := img.MediaType()
		if err != nil {
			t.Fatalf("MediaType() failed: %v", err)
		}

		if mediaType != ggcrtypes.OCIManifestSchema1 {
			t.Errorf("MediaType() = %q, want %q", mediaType, ggcrtypes.OCIManifestSchema1)
		}
	})

	// Verify layer is present
	t.Run("layer is present", func(t *testing.T) {
		layers, err := img.Layers()
		if err != nil {
			t.Fatalf("Layers() failed: %v", err)
		}

		if len(layers) != 1 {
			t.Errorf("Layers() count = %d, want 1", len(layers))
		}
	})

	// Verify ConfigFile returns error (artifacts don't have ConfigFile)
	t.Run("ConfigFile returns error", func(t *testing.T) {
		_, err := img.ConfigFile()
		if err == nil {
			t.Error("ConfigFile() should return error for artifacts, but got nil")
		}
	})

	// Verify Digest() matches RawManifest() digest (CRITICAL for registry push)
	t.Run("Digest matches RawManifest digest", func(t *testing.T) {
		digest, err := img.Digest()
		if err != nil {
			t.Fatalf("Digest() failed: %v", err)
		}

		rawManifest, err := img.RawManifest()
		if err != nil {
			t.Fatalf("RawManifest() failed: %v", err)
		}

		// Calculate expected digest from raw manifest bytes
		expectedDigest, _, err := v1.SHA256(bytes.NewReader(rawManifest))
		if err != nil {
			t.Fatalf("calculating manifest digest: %v", err)
		}

		if digest != expectedDigest {
			t.Errorf("Digest() = %v, but RawManifest digest = %v (MISMATCH - will cause registry push failure)", digest, expectedDigest)
		}
	})

	// Verify ConfigName() matches empty config hash
	t.Run("ConfigName matches empty config hash", func(t *testing.T) {
		configName, err := img.ConfigName()
		if err != nil {
			t.Fatalf("ConfigName() failed: %v", err)
		}

		// Calculate expected config hash
		emptyConfig := []byte("{}")
		expectedHash, _, err := v1.SHA256(bytes.NewReader(emptyConfig))
		if err != nil {
			t.Fatalf("calculating empty config hash: %v", err)
		}

		if configName != expectedHash {
			t.Errorf("ConfigName() = %v, want %v", configName, expectedHash)
		}
	})

	// Verify Size() matches RawManifest() length
	t.Run("Size matches RawManifest length", func(t *testing.T) {
		size, err := img.Size()
		if err != nil {
			t.Fatalf("Size() failed: %v", err)
		}

		rawManifest, err := img.RawManifest()
		if err != nil {
			t.Fatalf("RawManifest() failed: %v", err)
		}

		expectedSize := int64(len(rawManifest))
		if size != expectedSize {
			t.Errorf("Size() = %d, want %d", size, expectedSize)
		}
	})
}
