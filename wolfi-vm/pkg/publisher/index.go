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

	"chainguard.dev/apko/pkg/build/types"
	"github.com/chainguard-dev/clog"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

// staticIndex implements v1.ImageIndex for our custom multi-artifact index
type staticIndex struct {
	indexManifest *v1.IndexManifest
	images        []ImageWithMetadata
	subIndexes    map[v1.Hash]v1.ImageIndex // For nested indexes
}

// MediaType implements v1.ImageIndex
func (s *staticIndex) MediaType() (ggcrtypes.MediaType, error) {
	return s.indexManifest.MediaType, nil
}

// Digest implements v1.ImageIndex
func (s *staticIndex) Digest() (v1.Hash, error) {
	// Compute digest from the manifest JSON
	manifestBytes, err := s.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(manifestBytes))
	return h, err
}

// Size implements v1.ImageIndex
func (s *staticIndex) Size() (int64, error) {
	manifestBytes, err := s.RawManifest()
	if err != nil {
		return 0, err
	}
	return int64(len(manifestBytes)), nil
}

// IndexManifest implements v1.ImageIndex
func (s *staticIndex) IndexManifest() (*v1.IndexManifest, error) {
	return s.indexManifest, nil
}

// RawManifest implements v1.ImageIndex
func (s *staticIndex) RawManifest() ([]byte, error) {
	return json.Marshal(s.indexManifest)
}

// Image implements v1.ImageIndex - returns the image at the given hash
func (s *staticIndex) Image(h v1.Hash) (v1.Image, error) {
	for _, imgMeta := range s.images {
		digest, err := imgMeta.Image.Digest()
		if err != nil {
			return nil, err
		}
		if digest == h {
			return imgMeta.Image, nil
		}
	}
	return nil, fmt.Errorf("image not found: %s", h.String())
}

// ImageIndex implements v1.ImageIndex - returns the nested index at the given hash
func (s *staticIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	if s.subIndexes != nil {
		if idx, ok := s.subIndexes[h]; ok {
			return idx, nil
		}
	}
	return nil, fmt.Errorf("nested index not found: %s", h.String())
}

// checkIfExists checks if a reference already exists in the registry
// Returns true if it exists, false otherwise
func checkIfExists(ref name.Reference, remoteOpts ...remote.Option) bool {
	_, err := remote.Head(ref, remoteOpts...)
	return err == nil
}

// createDescriptor creates a v1.Descriptor with platform information and optional annotations
func createDescriptor(digest v1.Hash, size int64, mediaType ggcrtypes.MediaType, arch types.Architecture, annotations map[string]string) v1.Descriptor {
	descriptor := v1.Descriptor{
		MediaType: mediaType,
		Size:      size,
		Digest:    digest,
		Platform: &v1.Platform{
			Architecture: arch.ToOCIPlatform().Architecture,
			OS:           arch.ToOCIPlatform().OS,
		},
	}
	if len(annotations) > 0 {
		descriptor.Annotations = annotations
	}
	return descriptor
}

// createSubIndex creates a sub-index for a single architecture containing all artifact type images
func (p *Publisher) createSubIndex(images []ImageWithMetadata, arch types.Architecture) (v1.ImageIndex, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images provided for %s", arch.ToAPK())
	}

	// Build sub-index manifest for this architecture
	subIndexManifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Annotations: map[string]string{
			AnnotationBuildTime:    time.Now().Format(time.RFC3339),
			AnnotationArchitecture: arch.ToAPK(),
		},
	}

	// Add each artifact type image to the sub-index
	for _, imgMeta := range images {
		digest, err := imgMeta.Image.Digest()
		if err != nil {
			return nil, fmt.Errorf("computing digest for %s: %w", imgMeta.ArtifactType, err)
		}

		size, err := imgMeta.Image.Size()
		if err != nil {
			return nil, fmt.Errorf("computing size for %s: %w", imgMeta.ArtifactType, err)
		}

		mediaType, err := imgMeta.Image.MediaType()
		if err != nil {
			return nil, fmt.Errorf("getting media type for %s: %w", imgMeta.ArtifactType, err)
		}

		descriptor := createDescriptor(digest, size, mediaType, arch, map[string]string{
			AnnotationArtifactType: imgMeta.ArtifactType,
		})

		subIndexManifest.Manifests = append(subIndexManifest.Manifests, descriptor)
	}

	// Create the sub-index
	subIdx := &staticIndex{
		indexManifest: subIndexManifest,
		images:        images,
	}

	return subIdx, nil
}

// publishIndex is the publishing logic used by PublishMultiArch.
// It creates a nested OCI Image Index structure: top-level index points to per-architecture
// sub-indexes, which each contain all artifact type images for that architecture.
func (p *Publisher) publishIndex(ctx context.Context, images []ImageWithMetadata, ref name.Repository, tags []string, skipIfExists bool, remoteOpts []remote.Option) (string, error) {
	log := clog.FromContext(ctx)

	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
	}

	// Group images by architecture
	imagesByArch := make(map[types.Architecture][]ImageWithMetadata)
	for _, img := range images {
		imagesByArch[img.Arch] = append(imagesByArch[img.Arch], img)
	}

	// Publish all individual images first
	log.Infof("Publishing %d image(s)", len(images))
	for _, imgMeta := range images {
		imgDigest, err := imgMeta.Image.Digest()
		if err != nil {
			return "", fmt.Errorf("computing digest for %s/%s: %w", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, err)
		}
		imgRef := ref.Digest(imgDigest.String())

		// Check if image already exists
		if skipIfExists && checkIfExists(imgRef, remoteOpts...) {
			log.Infof("Image %s/%s already exists, skipping", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType)
			continue
		}

		if err := remote.Write(imgRef, imgMeta.Image, remoteOpts...); err != nil {
			return "", fmt.Errorf("publishing %s/%s: %w", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, err)
		}
		log.Infof("Published %s/%s: %s", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, imgRef.String())
	}

	// Create and publish sub-indexes for each architecture
	subIndexes := make(map[types.Architecture]v1.ImageIndex)

	for arch, archImages := range imagesByArch {
		log.Infof("Creating sub-index for %s with %d images", arch.ToAPK(), len(archImages))

		subIdx, err := p.createSubIndex(archImages, arch)
		if err != nil {
			return "", fmt.Errorf("creating sub-index for %s: %w", arch.ToAPK(), err)
		}

		subIdxDigest, err := subIdx.Digest()
		if err != nil {
			return "", fmt.Errorf("computing sub-index digest for %s: %w", arch.ToAPK(), err)
		}

		subIdxRef := ref.Digest(subIdxDigest.String())

		// Check if sub-index already exists
		if skipIfExists && checkIfExists(subIdxRef, remoteOpts...) {
			log.Infof("Sub-index for %s already exists, skipping", arch.ToAPK())
			subIndexes[arch] = subIdx
			continue
		}

		// Publish the sub-index
		if err := remote.WriteIndex(subIdxRef, subIdx, remoteOpts...); err != nil {
			return "", fmt.Errorf("publishing sub-index for %s: %w", arch.ToAPK(), err)
		}
		log.Infof("Published sub-index for %s: %s", arch.ToAPK(), subIdxRef.String())

		subIndexes[arch] = subIdx
	}

	// Create top-level index that points to sub-indexes
	topIndexManifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Annotations: map[string]string{
			AnnotationBuildTime: time.Now().Format(time.RFC3339),
		},
	}

	// Build hash-keyed map for staticIndex and populate top-level manifest
	subIndexesByHash := make(map[v1.Hash]v1.ImageIndex)
	for arch, subIdx := range subIndexes {
		subIdxDigest, err := subIdx.Digest()
		if err != nil {
			return "", fmt.Errorf("computing sub-index digest for %s: %w", arch.ToAPK(), err)
		}

		subIdxSize, err := subIdx.Size()
		if err != nil {
			return "", fmt.Errorf("computing sub-index size for %s: %w", arch.ToAPK(), err)
		}

		subIdxMediaType, err := subIdx.MediaType()
		if err != nil {
			return "", fmt.Errorf("getting sub-index media type for %s: %w", arch.ToAPK(), err)
		}

		descriptor := createDescriptor(subIdxDigest, subIdxSize, subIdxMediaType, arch, nil)

		topIndexManifest.Manifests = append(topIndexManifest.Manifests, descriptor)
		subIndexesByHash[subIdxDigest] = subIdx
		log.Infof("Added sub-index for %s to top-level index", arch.ToAPK())
	}

	idx := &staticIndex{
		indexManifest: topIndexManifest,
		subIndexes:    subIndexesByHash,
	}

	// Check if top-level index already exists
	h, err := idx.Digest()
	if err != nil {
		return "", fmt.Errorf("computing top-level index digest: %w", err)
	}
	indexDigest := ref.Digest(h.String())

	if skipIfExists && checkIfExists(indexDigest, remoteOpts...) {
		log.Infof("Top-level index already exists, skipping push: %s", indexDigest.String())

		// Still apply tags if requested
		if len(tags) > 0 {
			log.Infof("Applying tags to existing top-level index")
			if _, err := applyTagsToTarget(ctx, ref, idx, tags, remoteOpts); err != nil {
				return "", fmt.Errorf("tagging existing index: %w", err)
			}
		}

		return indexDigest.String(), nil
	}

	// Publish the top-level index by digest first
	if err := remote.WriteIndex(indexDigest, idx, remoteOpts...); err != nil {
		return "", fmt.Errorf("publishing top-level index: %w", err)
	}
	log.Infof("Published top-level index: %s", indexDigest.String())

	// Apply tags to the top-level index
	if _, err := applyTagsToTarget(ctx, ref, idx, tags, remoteOpts); err != nil {
		return "", fmt.Errorf("tagging top-level index: %w", err)
	}

	return indexDigest.String(), nil
}

// FetchExistingIndex attempts to fetch an existing OCI image index from the registry by tag.
// This is used for incremental multi-arch publishing where we want to add a new architecture
// to an existing multi-arch index rather than replacing it.
//
// Returns:
// - The existing index if found
// - nil if the tag doesn't exist (not an error - indicates first publish)
// - error only if there's a communication failure or other real problem
func (p *Publisher) FetchExistingIndex(ctx context.Context, repoRef string, tag string, remoteOpts []remote.Option) (v1.ImageIndex, error) {
	log := clog.FromContext(ctx)

	ref, err := name.NewRepository(repoRef)
	if err != nil {
		return nil, fmt.Errorf("parsing repository reference: %w", err)
	}

	tagRef := ref.Tag(tag)
	log.Infof("Checking for existing index at %s", tagRef.String())

	// Try to fetch the index
	idx, err := remote.Index(tagRef, remoteOpts...)
	if err != nil {
		// If we can't fetch the index, it likely doesn't exist yet (first publish)
		// This is not an error - just return nil to indicate no existing index
		log.Infof("No existing index found at %s (first publish or error: %v)", tagRef.String(), err)
		return nil, nil
	}

	log.Infof("Found existing index at %s", tagRef.String())
	return idx, nil
}

// MergeArchIntoIndex merges new architecture images into an existing multi-arch OCI index.
// This enables incremental multi-arch publishing where each architecture can be built and
// published separately without overwriting other architectures.
//
// The function:
// 1. Fetches existing sub-indexes from the registry for all architectures in the existing index
// 2. Creates new sub-indexes for the new architecture(s)
// 3. Combines all sub-indexes (existing + new) into a new top-level multi-arch index
// 4. Publishes all components (images, sub-indexes, top-level index)
//
// If an architecture already exists in the index, it will be replaced with the new version.
//
// Returns the digest of the newly published merged index.
func (p *Publisher) MergeArchIntoIndex(ctx context.Context, existingIndex v1.ImageIndex, newImages []ImageWithMetadata, ref name.Repository, tags []string, skipIfExists bool, remoteOpts []remote.Option) (string, error) {
	log := clog.FromContext(ctx)
	log.Infof("Merging %d new images into existing index", len(newImages))

	if len(newImages) == 0 {
		return "", fmt.Errorf("no new images provided for merge")
	}

	// Get the existing index manifest
	existingManifest, err := existingIndex.IndexManifest()
	if err != nil {
		return "", fmt.Errorf("getting existing index manifest: %w", err)
	}

	// Group new images by architecture
	newImagesByArch := make(map[types.Architecture][]ImageWithMetadata)
	for _, img := range newImages {
		newImagesByArch[img.Arch] = append(newImagesByArch[img.Arch], img)
	}

	// Publish all new individual images first
	log.Infof("Publishing %d new image(s)", len(newImages))
	for _, imgMeta := range newImages {
		imgDigest, err := imgMeta.Image.Digest()
		if err != nil {
			return "", fmt.Errorf("computing digest for %s/%s: %w", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, err)
		}
		imgRef := ref.Digest(imgDigest.String())

		if skipIfExists && checkIfExists(imgRef, remoteOpts...) {
			log.Infof("Image %s/%s already exists, skipping", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType)
			continue
		}

		if err := remote.Write(imgRef, imgMeta.Image, remoteOpts...); err != nil {
			return "", fmt.Errorf("publishing %s/%s: %w", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, err)
		}
		log.Infof("Published %s/%s: %s", imgMeta.Arch.ToAPK(), imgMeta.ArtifactType, imgRef.String())
	}

	// Track which architectures we're adding/replacing
	replacedArchs := make(map[types.Architecture]bool)
	for arch := range newImagesByArch {
		replacedArchs[arch] = true
	}

	// Build map of existing sub-indexes (excluding ones we're replacing)
	existingSubIndexes := make(map[types.Architecture]v1.Descriptor)
	subIndexesByHash := make(map[v1.Hash]v1.ImageIndex)

	for _, desc := range existingManifest.Manifests {
		if desc.Platform == nil {
			log.Warnf("Skipping descriptor without platform information")
			continue
		}

		// Parse the architecture from the descriptor
		arch := types.ParseArchitecture(desc.Platform.Architecture)

		// Skip if we're replacing this architecture
		if replacedArchs[arch] {
			log.Infof("Replacing existing sub-index for %s", arch.ToAPK())
			continue
		}

		// Fetch the existing sub-index from the registry
		subIdxRef := ref.Digest(desc.Digest.String())
		subIdx, err := remote.Index(subIdxRef, remoteOpts...)
		if err != nil {
			return "", fmt.Errorf("fetching existing sub-index for %s: %w", arch.ToAPK(), err)
		}

		existingSubIndexes[arch] = desc
		subIndexesByHash[desc.Digest] = subIdx
		log.Infof("Preserving existing sub-index for %s", arch.ToAPK())
	}

	// Create and publish new sub-indexes for new architectures
	newSubIndexes := make(map[types.Architecture]v1.ImageIndex)

	for arch, archImages := range newImagesByArch {
		log.Infof("Creating new sub-index for %s with %d images", arch.ToAPK(), len(archImages))

		subIdx, err := p.createSubIndex(archImages, arch)
		if err != nil {
			return "", fmt.Errorf("creating sub-index for %s: %w", arch.ToAPK(), err)
		}

		subIdxDigest, err := subIdx.Digest()
		if err != nil {
			return "", fmt.Errorf("computing sub-index digest for %s: %w", arch.ToAPK(), err)
		}

		subIdxRef := ref.Digest(subIdxDigest.String())

		if skipIfExists && checkIfExists(subIdxRef, remoteOpts...) {
			log.Infof("Sub-index for %s already exists, skipping", arch.ToAPK())
		} else {
			if err := remote.WriteIndex(subIdxRef, subIdx, remoteOpts...); err != nil {
				return "", fmt.Errorf("publishing sub-index for %s: %w", arch.ToAPK(), err)
			}
			log.Infof("Published sub-index for %s: %s", arch.ToAPK(), subIdxRef.String())
		}

		newSubIndexes[arch] = subIdx
		subIndexesByHash[subIdxDigest] = subIdx
	}

	// Create new top-level index with all sub-indexes (existing + new)
	topIndexManifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Annotations: map[string]string{
			AnnotationBuildTime: time.Now().Format(time.RFC3339),
		},
	}

	// Add existing sub-index descriptors (ones we're keeping)
	for arch, desc := range existingSubIndexes {
		topIndexManifest.Manifests = append(topIndexManifest.Manifests, desc)
		log.Infof("Added existing sub-index for %s to merged index", arch.ToAPK())
	}

	// Add new sub-index descriptors
	for arch, subIdx := range newSubIndexes {
		subIdxDigest, err := subIdx.Digest()
		if err != nil {
			return "", fmt.Errorf("computing sub-index digest for %s: %w", arch.ToAPK(), err)
		}

		subIdxSize, err := subIdx.Size()
		if err != nil {
			return "", fmt.Errorf("computing sub-index size for %s: %w", arch.ToAPK(), err)
		}

		subIdxMediaType, err := subIdx.MediaType()
		if err != nil {
			return "", fmt.Errorf("getting sub-index media type for %s: %w", arch.ToAPK(), err)
		}

		descriptor := createDescriptor(subIdxDigest, subIdxSize, subIdxMediaType, arch, nil)
		topIndexManifest.Manifests = append(topIndexManifest.Manifests, descriptor)
		log.Infof("Added new sub-index for %s to merged index", arch.ToAPK())
	}

	// Create the merged top-level index
	mergedIdx := &staticIndex{
		indexManifest: topIndexManifest,
		subIndexes:    subIndexesByHash,
	}

	// Publish the merged top-level index
	h, err := mergedIdx.Digest()
	if err != nil {
		return "", fmt.Errorf("computing merged index digest: %w", err)
	}
	indexDigest := ref.Digest(h.String())

	if err := remote.WriteIndex(indexDigest, mergedIdx, remoteOpts...); err != nil {
		return "", fmt.Errorf("publishing merged index: %w", err)
	}
	log.Infof("Published merged multi-arch index: %s", indexDigest.String())

	// Apply tags to the merged index
	if len(tags) > 0 {
		if _, err := applyTagsToTarget(ctx, ref, mergedIdx, tags, remoteOpts); err != nil {
			return "", fmt.Errorf("tagging merged index: %w", err)
		}
	}

	return indexDigest.String(), nil
}

// mergePublishedIndexes merges two already-published OCI indexes from the registry.
// This is used to handle race conditions where another process publishes an index
// between our publish and tag operations. The function fetches both indexes,
// combines their sub-index manifests (preferring newIndexDigest for overlapping architectures),
// and publishes a merged result.
//
// Returns the digest of the newly published merged index.
func (p *Publisher) mergePublishedIndexes(ctx context.Context, newIndexDigest string, existingIndexDigest string, ref name.Repository, remoteOpts []remote.Option) (string, error) {
	log := clog.FromContext(ctx)
	log.Infof("Merging published indexes: new=%s, existing=%s", newIndexDigest, existingIndexDigest)

	// If they're the same, no merge needed
	if newIndexDigest == existingIndexDigest {
		log.Infof("Indexes are identical, no merge needed")
		return newIndexDigest, nil
	}

	// Fetch both indexes from the registry
	newIdxRef, err := name.NewDigest(newIndexDigest)
	if err != nil {
		return "", fmt.Errorf("parsing new index digest: %w", err)
	}

	existingIdxRef, err := name.NewDigest(existingIndexDigest)
	if err != nil {
		return "", fmt.Errorf("parsing existing index digest: %w", err)
	}

	newIdx, err := remote.Index(newIdxRef, remoteOpts...)
	if err != nil {
		return "", fmt.Errorf("fetching new index: %w", err)
	}

	existingIdx, err := remote.Index(existingIdxRef, remoteOpts...)
	if err != nil {
		return "", fmt.Errorf("fetching existing index: %w", err)
	}

	// Get manifests from both indexes
	newManifest, err := newIdx.IndexManifest()
	if err != nil {
		return "", fmt.Errorf("getting new index manifest: %w", err)
	}

	existingManifest, err := existingIdx.IndexManifest()
	if err != nil {
		return "", fmt.Errorf("getting existing index manifest: %w", err)
	}

	// Track which architectures are in the new index
	newArchs := make(map[types.Architecture]v1.Descriptor)
	subIndexesByHash := make(map[v1.Hash]v1.ImageIndex)

	for _, desc := range newManifest.Manifests {
		if desc.Platform == nil {
			log.Warnf("Skipping descriptor without platform in new index")
			continue
		}
		arch := types.ParseArchitecture(desc.Platform.Architecture)
		newArchs[arch] = desc

		// Fetch the sub-index for later reconstruction
		subIdxRef := ref.Digest(desc.Digest.String())
		subIdx, err := remote.Index(subIdxRef, remoteOpts...)
		if err != nil {
			return "", fmt.Errorf("fetching new sub-index for %s: %w", arch.ToAPK(), err)
		}
		subIndexesByHash[desc.Digest] = subIdx
		log.Infof("Including new sub-index for %s", arch.ToAPK())
	}

	// Add existing architectures that aren't in the new index
	for _, desc := range existingManifest.Manifests {
		if desc.Platform == nil {
			log.Warnf("Skipping descriptor without platform in existing index")
			continue
		}
		arch := types.ParseArchitecture(desc.Platform.Architecture)

		// Skip if we already have this architecture from the new index
		if _, exists := newArchs[arch]; exists {
			log.Infof("Replacing existing sub-index for %s with new version", arch.ToAPK())
			continue
		}

		// Fetch the sub-index for later reconstruction
		subIdxRef := ref.Digest(desc.Digest.String())
		subIdx, err := remote.Index(subIdxRef, remoteOpts...)
		if err != nil {
			return "", fmt.Errorf("fetching existing sub-index for %s: %w", arch.ToAPK(), err)
		}
		subIndexesByHash[desc.Digest] = subIdx
		newArchs[arch] = desc
		log.Infof("Preserving existing sub-index for %s", arch.ToAPK())
	}

	// Build merged top-level index manifest
	mergedManifest := &v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     ggcrtypes.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
		Annotations: map[string]string{
			AnnotationBuildTime: time.Now().Format(time.RFC3339),
		},
	}

	// Add all descriptors to the merged manifest
	for _, desc := range newArchs {
		mergedManifest.Manifests = append(mergedManifest.Manifests, desc)
	}

	// Create the merged index
	mergedIdx := &staticIndex{
		indexManifest: mergedManifest,
		subIndexes:    subIndexesByHash,
	}

	// Publish the merged index
	h, err := mergedIdx.Digest()
	if err != nil {
		return "", fmt.Errorf("computing merged index digest: %w", err)
	}
	mergedDigest := ref.Digest(h.String())

	if err := remote.WriteIndex(mergedDigest, mergedIdx, remoteOpts...); err != nil {
		return "", fmt.Errorf("publishing merged index: %w", err)
	}

	log.Infof("Published merged index with %d architectures: %s", len(newArchs), mergedDigest.String())
	return mergedDigest.String(), nil
}
