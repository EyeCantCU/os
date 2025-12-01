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
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"chainguard.dev/apko/pkg/build/types"
)

// TestIsSecureBootFile tests detection of secure boot related files.
// Validates matching of "uefi-*" prefix and .auth, .esl, .fd, .bin extensions.
func TestIsSecureBootFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		// UEFI prefix patterns - should match
		{filename: "uefi-data.json", want: true},
		{filename: "uefi-vars.fd", want: true},
		{filename: "uefi-code.bin", want: true},
		{filename: "uefi-", want: true}, // Edge case: just prefix
		{filename: "uefi-something.txt", want: true},

		// Extension patterns - should match
		{filename: "file.auth", want: true},
		{filename: "secure.esl", want: true},
		{filename: "data.fd", want: true},
		{filename: "boot.bin", want: true},
		{filename: "PK.auth", want: true},
		{filename: "KEK.esl", want: true},

		// Should not match
		{filename: "disk.raw.v1", want: false},
		{filename: "disk.qcow2.v1", want: false},
		{filename: "regular.txt", want: false},
		{filename: "apko.tar.gz", want: false},
		{filename: "sbom.json", want: false},
		{filename: "", want: false},
		{filename: ".auth", want: true},    // Just extension matches
		{filename: "auth", want: false},    // No dot
		{filename: "not-uefi.txt", want: false},
		{filename: "UEFI-data.json", want: false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := isSecureBootFile(tt.filename)
			if got != tt.want {
				t.Errorf("isSecureBootFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// TestCopyICWithArtifactType tests deep copying of ImageConfiguration with artifact type annotation.
// Validates annotation preservation, artifact type overwriting, and proper deep copy behavior.
func TestCopyICWithArtifactType(t *testing.T) {
	tests := []struct {
		name         string
		base         types.ImageConfiguration
		artifactType string
		wantType     string
	}{
		{
			name: "empty config with artifact type",
			base: types.ImageConfiguration{
				Annotations: map[string]string{},
			},
			artifactType: "disk.raw.v1",
			wantType:     "disk.raw.v1",
		},
		{
			name: "config with existing annotations",
			base: types.ImageConfiguration{
				Annotations: map[string]string{
					"org.opencontainers.image.created": "2024-01-01T00:00:00Z",
					"org.opencontainers.image.version": "1.0.0",
				},
			},
			artifactType: "disk.qcow2.v1",
			wantType:     "disk.qcow2.v1",
		},
		{
			name: "overwrite existing artifact type",
			base: types.ImageConfiguration{
				Annotations: map[string]string{
					AnnotationArtifactType: "old-type",
					"other-key":            "other-value",
				},
			},
			artifactType: "disk.vmdk.v1",
			wantType:     "disk.vmdk.v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Count original annotations
			origCount := len(tt.base.Annotations)

			// Copy with new artifact type
			result := copyICWithArtifactType(tt.base, tt.artifactType)

			// Verify artifact type annotation is set correctly
			if result.Annotations[AnnotationArtifactType] != tt.wantType {
				t.Errorf("result.Annotations[%q] = %q, want %q",
					AnnotationArtifactType, result.Annotations[AnnotationArtifactType], tt.wantType)
			}

			// Verify all original annotations are copied (except artifact type which may be overwritten)
			for k, v := range tt.base.Annotations {
				if k == AnnotationArtifactType {
					continue // Skip artifact type, we expect it to be overwritten
				}
				if result.Annotations[k] != v {
					t.Errorf("result.Annotations[%q] = %q, want %q", k, result.Annotations[k], v)
				}
			}

			// Verify we have at least the same number of annotations (or one more if artifact type was new)
			if len(result.Annotations) < origCount {
				t.Errorf("result has %d annotations, want at least %d", len(result.Annotations), origCount)
			}

			// Verify deep copy: modifying result should not affect original
			result.Annotations["test-key"] = "test-value"
			if _, exists := tt.base.Annotations["test-key"]; exists {
				t.Error("modifying result annotations affected original (not a deep copy)")
			}
		})
	}
}

// TestWrapFilesInTar tests wrapping files in tar archive format.
// Validates tar structure, file modes (0644), headers, and error handling for nonexistent files.
func TestWrapFilesInTar(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) []string
		wantErr   bool
		wantFiles []string // Expected filenames in tar
	}{
		{
			name: "single file",
			setupFunc: func(t *testing.T) []string {
				dir := t.TempDir()
				file := filepath.Join(dir, "test.txt")
				if err := os.WriteFile(file, []byte("hello world"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return []string{file}
			},
			wantErr:   false,
			wantFiles: []string{"test.txt"},
		},
		{
			name: "multiple files",
			setupFunc: func(t *testing.T) []string {
				dir := t.TempDir()
				files := []string{}
				for _, name := range []string{"file1.txt", "file2.json", "file3.bin"} {
					file := filepath.Join(dir, name)
					if err := os.WriteFile(file, []byte("content of "+name), 0644); err != nil {
						t.Fatalf("failed to create test file: %v", err)
					}
					files = append(files, file)
				}
				return files
			},
			wantErr:   false,
			wantFiles: []string{"file1.txt", "file2.json", "file3.bin"},
		},
		{
			name: "nonexistent file",
			setupFunc: func(t *testing.T) []string {
				return []string{"/nonexistent/path/to/file.txt"}
			},
			wantErr:   true,
			wantFiles: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := tt.setupFunc(t)

			rc, err := wrapFilesInTar(paths)
			if err != nil {
				t.Fatalf("wrapFilesInTar() unexpected immediate error: %v", err)
			}
			if rc == nil {
				t.Fatal("wrapFilesInTar() returned nil ReadCloser")
			}
			defer rc.Close()

			if tt.wantErr {
				// For error cases, we expect the error to occur when reading
				_, readErr := io.ReadAll(rc)
				if readErr == nil {
					t.Error("wrapFilesInTar() expected error when reading but got none")
				}
				return
			}

			// Read and verify tar contents
			tr := tar.NewReader(rc)
			foundFiles := []string{}
			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("error reading tar: %v", err)
				}
				foundFiles = append(foundFiles, hdr.Name)

				// Verify mode
				if hdr.Mode != 0644 {
					t.Errorf("file %s has mode %o, want 0644", hdr.Name, hdr.Mode)
				}

				// Verify size is reasonable (> 0 for our test files)
				if hdr.Size <= 0 {
					t.Errorf("file %s has size %d, want > 0", hdr.Name, hdr.Size)
				}
			}

			// Verify all expected files were found
			if len(foundFiles) != len(tt.wantFiles) {
				t.Errorf("tar contains %d files, want %d", len(foundFiles), len(tt.wantFiles))
			}

			for i, want := range tt.wantFiles {
				if i >= len(foundFiles) {
					t.Errorf("missing file in tar: %s", want)
					continue
				}
				if foundFiles[i] != want {
					t.Errorf("tar file[%d] = %s, want %s", i, foundFiles[i], want)
				}
			}
		})
	}
}
