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
	"encoding/json"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

// mockImage is a minimal implementation of v1.Image for testing
type mockImage struct {
	digest v1.Hash
	size   int64
}

func (m *mockImage) Digest() (v1.Hash, error) {
	return m.digest, nil
}

func (m *mockImage) Size() (int64, error) {
	return m.size, nil
}

func (m *mockImage) MediaType() (ggcrtypes.MediaType, error) {
	return ggcrtypes.OCIManifestSchema1, nil
}

func (m *mockImage) Layers() ([]v1.Layer, error)       { return nil, nil }
func (m *mockImage) Manifest() (*v1.Manifest, error)   { return nil, nil }
func (m *mockImage) RawManifest() ([]byte, error)      { return nil, nil }
func (m *mockImage) ConfigName() (v1.Hash, error)      { return v1.Hash{}, nil }
func (m *mockImage) ConfigFile() (*v1.ConfigFile, error) { return nil, nil }
func (m *mockImage) RawConfigFile() ([]byte, error)    { return nil, nil }
func (m *mockImage) LayerByDigest(v1.Hash) (v1.Layer, error) { return nil, nil }
func (m *mockImage) LayerByDiffID(v1.Hash) (v1.Layer, error) { return nil, nil }

// mockImageIndex is a minimal implementation of v1.ImageIndex for testing
type mockImageIndex struct {
	digest v1.Hash
	size   int64
}

func (m *mockImageIndex) Digest() (v1.Hash, error) {
	return m.digest, nil
}

func (m *mockImageIndex) Size() (int64, error) {
	return m.size, nil
}

func (m *mockImageIndex) MediaType() (ggcrtypes.MediaType, error) {
	return ggcrtypes.OCIImageIndex, nil
}

func (m *mockImageIndex) IndexManifest() (*v1.IndexManifest, error) { return nil, nil }
func (m *mockImageIndex) RawManifest() ([]byte, error)              { return nil, nil }
func (m *mockImageIndex) Image(v1.Hash) (v1.Image, error)           { return nil, nil }
func (m *mockImageIndex) ImageIndex(v1.Hash) (v1.ImageIndex, error) { return nil, nil }

// TestCreateDescriptor tests OCI descriptor creation with digest, size, platform,
// and annotation handling. Validates platform conversion (x86_64→amd64, aarch64→arm64).
func TestCreateDescriptor(t *testing.T) {
	digest, _ := v1.NewHash("sha256:0000000000000000000000000000000000000000000000000000000000000000")

	tests := []struct {
		name        string
		digest      v1.Hash
		size        int64
		mediaType   ggcrtypes.MediaType
		arch        types.Architecture
		annotations map[string]string
		wantArch    string
		wantOS      string
		wantAnnots  int
	}{
		{
			name:       "basic descriptor - x86_64",
			digest:     digest,
			size:       1024,
			mediaType:  ggcrtypes.OCIManifestSchema1,
			arch:       types.ParseArchitecture("x86_64"),
			annotations: nil,
			wantArch:   "amd64",
			wantOS:     "linux",
			wantAnnots: 0,
		},
		{
			name:       "basic descriptor - aarch64",
			digest:     digest,
			size:       2048,
			mediaType:  ggcrtypes.OCIManifestSchema1,
			arch:       types.ParseArchitecture("aarch64"),
			annotations: nil,
			wantArch:   "arm64",
			wantOS:     "linux",
			wantAnnots: 0,
		},
		{
			name:      "descriptor with annotations",
			digest:    digest,
			size:      512,
			mediaType: ggcrtypes.OCIImageIndex,
			arch:      types.ParseArchitecture("x86_64"),
			annotations: map[string]string{
				AnnotationArtifactType: "disk.raw.v1",
				"custom.annotation":    "value",
			},
			wantArch:   "amd64",
			wantOS:     "linux",
			wantAnnots: 2,
		},
		{
			name:       "descriptor with empty annotations map",
			digest:     digest,
			size:       256,
			mediaType:  ggcrtypes.OCIManifestSchema1,
			arch:       types.ParseArchitecture("aarch64"),
			annotations: map[string]string{},
			wantArch:   "arm64",
			wantOS:     "linux",
			wantAnnots: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := createDescriptor(tt.digest, tt.size, tt.mediaType, tt.arch, tt.annotations)

			if desc.Digest != tt.digest {
				t.Errorf("Digest = %v, want %v", desc.Digest, tt.digest)
			}
			if desc.Size != tt.size {
				t.Errorf("Size = %d, want %d", desc.Size, tt.size)
			}
			if desc.MediaType != tt.mediaType {
				t.Errorf("MediaType = %v, want %v", desc.MediaType, tt.mediaType)
			}
			if desc.Platform == nil {
				t.Fatal("Platform is nil")
			}
			if desc.Platform.Architecture != tt.wantArch {
				t.Errorf("Platform.Architecture = %s, want %s", desc.Platform.Architecture, tt.wantArch)
			}
			if desc.Platform.OS != tt.wantOS {
				t.Errorf("Platform.OS = %s, want %s", desc.Platform.OS, tt.wantOS)
			}
			if len(desc.Annotations) != tt.wantAnnots {
				t.Errorf("Annotations length = %d, want %d", len(desc.Annotations), tt.wantAnnots)
			}
			if tt.wantAnnots > 0 {
				for k, v := range tt.annotations {
					if desc.Annotations[k] != v {
						t.Errorf("Annotations[%q] = %q, want %q", k, desc.Annotations[k], v)
					}
				}
			}
		})
	}
}

// TestStaticIndex_MediaType tests media type retrieval from static index.
// Validates OCIImageIndex and DockerManifestList media types.
func TestStaticIndex_MediaType(t *testing.T) {
	tests := []struct {
		name      string
		mediaType ggcrtypes.MediaType
	}{
		{
			name:      "OCIImageIndex",
			mediaType: ggcrtypes.OCIImageIndex,
		},
		{
			name:      "DockerManifestList",
			mediaType: ggcrtypes.DockerManifestList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &staticIndex{
				indexManifest: &v1.IndexManifest{
					MediaType: tt.mediaType,
				},
			}

			got, err := idx.MediaType()
			if err != nil {
				t.Fatalf("MediaType() error = %v", err)
			}
			if got != tt.mediaType {
				t.Errorf("MediaType() = %v, want %v", got, tt.mediaType)
			}
		})
	}
}

// TestStaticIndex_IndexManifest tests index manifest structure retrieval
// including SchemaVersion, MediaType, and Annotations.
func TestStaticIndex_IndexManifest(t *testing.T) {
	manifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Annotations: map[string]string{
			"test": "value",
		},
	}

	idx := &staticIndex{
		indexManifest: manifest,
	}

	got, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest() error = %v", err)
	}
	if got != manifest {
		t.Errorf("IndexManifest() returned different pointer")
	}
	if got.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2", got.SchemaVersion)
	}
	if got.Annotations["test"] != "value" {
		t.Errorf("Annotations[test] = %s, want value", got.Annotations["test"])
	}
}

// TestStaticIndex_RawManifest tests JSON serialization of index manifests.
// Validates JSON parsing and structure preservation.
func TestStaticIndex_RawManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest *v1.IndexManifest
		validate func(t *testing.T, raw []byte)
	}{
		{
			name: "basic manifest",
			manifest: &v1.IndexManifest{
				SchemaVersion: 2,
				MediaType:     ggcrtypes.OCIImageIndex,
				Manifests:     []v1.Descriptor{},
			},
			validate: func(t *testing.T, raw []byte) {
				var parsed v1.IndexManifest
				if err := json.Unmarshal(raw, &parsed); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				if parsed.SchemaVersion != 2 {
					t.Errorf("SchemaVersion = %d, want 2", parsed.SchemaVersion)
				}
			},
		},
		{
			name: "manifest with annotations",
			manifest: &v1.IndexManifest{
				SchemaVersion: 2,
				MediaType:     ggcrtypes.OCIImageIndex,
				Manifests:     []v1.Descriptor{},
				Annotations: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
			},
			validate: func(t *testing.T, raw []byte) {
				var parsed v1.IndexManifest
				if err := json.Unmarshal(raw, &parsed); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				if len(parsed.Annotations) != 2 {
					t.Errorf("Annotations length = %d, want 2", len(parsed.Annotations))
				}
				if parsed.Annotations["foo"] != "bar" {
					t.Errorf("Annotations[foo] = %s, want bar", parsed.Annotations["foo"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &staticIndex{
				indexManifest: tt.manifest,
			}

			raw, err := idx.RawManifest()
			if err != nil {
				t.Fatalf("RawManifest() error = %v", err)
			}
			if len(raw) == 0 {
				t.Error("RawManifest() returned empty bytes")
			}

			tt.validate(t, raw)
		})
	}
}

// TestStaticIndex_Size tests size calculation of index manifest.
// Validates size matches RawManifest length.
func TestStaticIndex_Size(t *testing.T) {
	manifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
	}

	idx := &staticIndex{
		indexManifest: manifest,
	}

	size, err := idx.Size()
	if err != nil {
		t.Fatalf("Size() error = %v", err)
	}

	// Verify size matches the length of RawManifest
	raw, err := idx.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest() error = %v", err)
	}

	if size != int64(len(raw)) {
		t.Errorf("Size() = %d, want %d (length of RawManifest)", size, int64(len(raw)))
	}

	if size <= 0 {
		t.Errorf("Size() = %d, want > 0", size)
	}
}

// TestStaticIndex_Digest tests SHA256 digest computation for index manifests.
// Validates digest is computed from RawManifest and changes when manifest changes.
func TestStaticIndex_Digest(t *testing.T) {
	manifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
	}

	idx := &staticIndex{
		indexManifest: manifest,
	}

	digest, err := idx.Digest()
	if err != nil {
		t.Fatalf("Digest() error = %v", err)
	}

	// Verify digest is computed from RawManifest
	raw, err := idx.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest() error = %v", err)
	}

	expectedDigest, _, err := v1.SHA256(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("SHA256() error = %v", err)
	}

	if digest != expectedDigest {
		t.Errorf("Digest() = %v, want %v", digest, expectedDigest)
	}

	// Verify digest changes when manifest changes
	manifest.Annotations = map[string]string{"new": "annotation"}
	idx2 := &staticIndex{
		indexManifest: manifest,
	}

	digest2, err := idx2.Digest()
	if err != nil {
		t.Fatalf("Digest() error = %v", err)
	}

	if digest == digest2 {
		t.Error("Digest() should change when manifest changes")
	}
}

// TestStaticIndex_Image tests image lookup by digest in static index.
// Validates finding images by hash and handling missing images.
func TestStaticIndex_Image(t *testing.T) {
	digest1, _ := v1.NewHash("sha256:1111111111111111111111111111111111111111111111111111111111111111")
	digest2, _ := v1.NewHash("sha256:2222222222222222222222222222222222222222222222222222222222222222")
	digest3, _ := v1.NewHash("sha256:3333333333333333333333333333333333333333333333333333333333333333")

	img1 := &mockImage{digest: digest1, size: 100}
	img2 := &mockImage{digest: digest2, size: 200}

	tests := []struct {
		name       string
		images     []ImageWithMetadata
		lookupHash v1.Hash
		wantFound  bool
		wantErr    bool
	}{
		{
			name: "image found - first image",
			images: []ImageWithMetadata{
				{Image: img1, ArtifactType: "disk.raw.v1"},
				{Image: img2, ArtifactType: "disk.qcow2.v1"},
			},
			lookupHash: digest1,
			wantFound:  true,
			wantErr:    false,
		},
		{
			name: "image found - second image",
			images: []ImageWithMetadata{
				{Image: img1, ArtifactType: "disk.raw.v1"},
				{Image: img2, ArtifactType: "disk.qcow2.v1"},
			},
			lookupHash: digest2,
			wantFound:  true,
			wantErr:    false,
		},
		{
			name: "image not found",
			images: []ImageWithMetadata{
				{Image: img1, ArtifactType: "disk.raw.v1"},
				{Image: img2, ArtifactType: "disk.qcow2.v1"},
			},
			lookupHash: digest3,
			wantFound:  false,
			wantErr:    true,
		},
		{
			name:       "empty images list",
			images:     []ImageWithMetadata{},
			lookupHash: digest1,
			wantFound:  false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &staticIndex{
				indexManifest: &v1.IndexManifest{},
				images:        tt.images,
			}

			img, err := idx.Image(tt.lookupHash)

			if (err != nil) != tt.wantErr {
				t.Errorf("Image() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantFound {
				if img == nil {
					t.Error("Image() returned nil, want non-nil image")
					return
				}
				gotDigest, _ := img.Digest()
				if gotDigest != tt.lookupHash {
					t.Errorf("Image() digest = %v, want %v", gotDigest, tt.lookupHash)
				}
			} else {
				if img != nil {
					t.Error("Image() returned non-nil, want nil")
				}
			}
		})
	}
}

// TestStaticIndex_ImageIndex tests sub-index lookup by digest in static index.
// Validates finding nested image indexes and handling nil/empty subIndexes maps.
func TestStaticIndex_ImageIndex(t *testing.T) {
	digest1, _ := v1.NewHash("sha256:1111111111111111111111111111111111111111111111111111111111111111")
	digest2, _ := v1.NewHash("sha256:2222222222222222222222222222222222222222222222222222222222222222")
	digest3, _ := v1.NewHash("sha256:3333333333333333333333333333333333333333333333333333333333333333")

	subIdx1 := &mockImageIndex{digest: digest1, size: 500}
	subIdx2 := &mockImageIndex{digest: digest2, size: 600}

	tests := []struct {
		name        string
		subIndexes  map[v1.Hash]v1.ImageIndex
		lookupHash  v1.Hash
		wantFound   bool
		wantErr     bool
	}{
		{
			name: "index found - first index",
			subIndexes: map[v1.Hash]v1.ImageIndex{
				digest1: subIdx1,
				digest2: subIdx2,
			},
			lookupHash: digest1,
			wantFound:  true,
			wantErr:    false,
		},
		{
			name: "index found - second index",
			subIndexes: map[v1.Hash]v1.ImageIndex{
				digest1: subIdx1,
				digest2: subIdx2,
			},
			lookupHash: digest2,
			wantFound:  true,
			wantErr:    false,
		},
		{
			name: "index not found",
			subIndexes: map[v1.Hash]v1.ImageIndex{
				digest1: subIdx1,
			},
			lookupHash: digest3,
			wantFound:  false,
			wantErr:    true,
		},
		{
			name:       "nil subIndexes map",
			subIndexes: nil,
			lookupHash: digest1,
			wantFound:  false,
			wantErr:    true,
		},
		{
			name:       "empty subIndexes map",
			subIndexes: map[v1.Hash]v1.ImageIndex{},
			lookupHash: digest1,
			wantFound:  false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := &staticIndex{
				indexManifest: &v1.IndexManifest{},
				subIndexes:    tt.subIndexes,
			}

			subIdx, err := idx.ImageIndex(tt.lookupHash)

			if (err != nil) != tt.wantErr {
				t.Errorf("ImageIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantFound {
				if subIdx == nil {
					t.Error("ImageIndex() returned nil, want non-nil index")
					return
				}
				gotDigest, _ := subIdx.Digest()
				if gotDigest != tt.lookupHash {
					t.Errorf("ImageIndex() digest = %v, want %v", gotDigest, tt.lookupHash)
				}
			} else {
				if subIdx != nil {
					t.Error("ImageIndex() returned non-nil, want nil")
				}
			}
		})
	}
}
