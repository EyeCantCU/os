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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"chainguard.dev/apko/pkg/build/types"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

// wrapFilesInTar wraps one or more files in a tar archive and returns a ReadCloser
func wrapFilesInTar(paths []string) (io.ReadCloser, error) {
	// Create a pipe so we can stream the tar creation
	pr, pw := io.Pipe()

	go func() {
		var err error
		defer func() {
			pw.CloseWithError(err)
		}()

		// Create tar writer
		tw := tar.NewWriter(pw)
		defer tw.Close()

		// Add each file to the tar
		for _, path := range paths {
			// Get file info for tar header
			var fileInfo os.FileInfo
			fileInfo, err = os.Stat(path)
			if err != nil {
				err = fmt.Errorf("stating file %s: %w", path, err)
				return
			}

			// Open the source file
			var f *os.File
			f, err = os.Open(path)
			if err != nil {
				err = fmt.Errorf("opening file %s: %w", path, err)
				return
			}

			// Write tar header
			hdr := &tar.Header{
				Name:    filepath.Base(path),
				Mode:    0644,
				Size:    fileInfo.Size(),
				ModTime: fileInfo.ModTime(),
			}
			if err = tw.WriteHeader(hdr); err != nil {
				f.Close()
				err = fmt.Errorf("writing tar header for %s: %w", path, err)
				return
			}

			// Copy file contents into tar
			if _, err = io.Copy(tw, f); err != nil {
				f.Close()
				err = fmt.Errorf("copying file %s to tar: %w", path, err)
				return
			}
			f.Close()
		}
	}()

	return pr, nil
}

// isSecureBootFile returns true if the filename is a UEFI/secure boot file
func isSecureBootFile(filename string) bool {
	// Match files starting with "uefi-" (e.g., uefi-data.json)
	if matched, _ := filepath.Match("uefi-*", filename); matched {
		return true
	}

	// Match by extension
	ext := filepath.Ext(filename)
	switch ext {
	case ".auth", ".esl", ".fd", ".bin":
		return true
	}

	return false
}

// copyICWithArtifactType creates a copy of the base ImageConfiguration with a new artifact type annotation
func copyICWithArtifactType(base types.ImageConfiguration, artifactType string) types.ImageConfiguration {
	ic := types.ImageConfiguration{
		Annotations: make(map[string]string),
	}
	for k, v := range base.Annotations {
		ic.Annotations[k] = v
	}
	ic.Annotations[AnnotationArtifactType] = artifactType
	return ic
}

// loadApkoTarball loads the apko rootfs tarball as a layer from the given file path
func (p *Publisher) loadApkoTarball(tarballPath string) (v1.Layer, error) {
	if tarballPath == "" {
		// No apko tarball specified, return nil
		return nil, nil
	}

	layer, err := tarball.LayerFromFile(tarballPath, tarball.WithMediaType(MediaTypeVMRootFS))
	if err != nil {
		return nil, fmt.Errorf("loading apko tarball %s: %w", tarballPath, err)
	}
	return layer, nil
}

// createMultiFileLayer creates a compressed layer from one or more files.
// All files are combined into a single tar archive.
func (p *Publisher) createMultiFileLayer(paths []string, mediaType ggcrtypes.MediaType) (v1.Layer, error) {
	opener := func() (io.ReadCloser, error) {
		return wrapFilesInTar(paths)
	}
	return tarball.LayerFromOpener(opener, tarball.WithMediaType(mediaType))
}
