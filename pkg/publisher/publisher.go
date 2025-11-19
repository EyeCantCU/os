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
	"context"
	"fmt"

	"chainguard.dev/apko/pkg/build/types"
	"github.com/chainguard-dev/clog"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Publisher handles publishing VM artifacts to an OCI registry
type Publisher struct {
	remoteOpts []remote.Option
}

func New(remoteOpts ...remote.Option) *Publisher {
	return &Publisher{
		remoteOpts: remoteOpts,
	}
}

// publishMultiArch publishes VM artifacts for one or more architectures using apko's OCI functions.
// Creates separate images for each artifact type and architecture, then publishes them in an index.
// Returns the digest reference of the published index.
// This is an internal function used by PublishMultiCloud.
func (p *Publisher) publishMultiArch(ctx context.Context, artifactsByArch map[types.Architecture]*Artifacts, repoRef string, opts *PublishOptions) (string, error) {
	if opts == nil {
		opts = &PublishOptions{}
	}

	if len(artifactsByArch) == 0 {
		return "", fmt.Errorf("no artifacts provided")
	}

	log := clog.FromContext(ctx)
	log.Infof("Publishing image with %d architecture(s)", len(artifactsByArch))

	ref, err := name.NewRepository(repoRef)
	if err != nil {
		return "", fmt.Errorf("parsing repository reference: %w", err)
	}

	remoteOpts := append([]remote.Option{remote.WithContext(ctx)}, p.remoteOpts...)

	// Build images for each architecture and artifact type (in memory)
	var allImages []ImageWithMetadata
	for arch, artifacts := range artifactsByArch {
		log.Infof("Building images for %s", arch.ToAPK())

		if err := artifacts.Validate(); err != nil {
			return "", fmt.Errorf("validating artifacts for %s: %w", arch.ToAPK(), err)
		}

		images, err := p.createImages(ctx, artifacts)
		if err != nil {
			return "", fmt.Errorf("creating images for %s: %w", arch.ToAPK(), err)
		}
		allImages = append(allImages, images...)
	}

	// Publish indexes (tags are applied by PublishMultiCloud)
	digestRef, err := p.publishIndex(ctx, allImages, ref, []string{}, opts.SkipIfExists, remoteOpts)
	if err != nil {
		return "", err
	}

	// Sign and attest if requested (multi-arch version)
	if opts.SignAndAttest {
		if err := p.SignAndAttestMultiArch(ctx, digestRef, artifactsByArch, opts.AttestationKeyRef, opts.SkipTransparencyLog); err != nil {
			return "", fmt.Errorf("signing and attesting multi-arch: %w", err)
		}
	}

	return digestRef, nil
}

// PublishSingleConfig publishes a single VM configuration to an OCI registry using a publish config.
// This is a simplified version of PublishMultiCloud for config-driven publishing.
// It loads artifacts from output directories, applies config-specified tags, and publishes as a multi-arch index.
// Returns a list of all applied tags.
func (p *Publisher) PublishSingleConfig(ctx context.Context, config *PublishConfig, outputDirs map[types.Architecture]string, repoRef string, opts *PublishOptions) ([]string, error) {
	if opts == nil {
		opts = &PublishOptions{}
	}

	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if len(outputDirs) == 0 {
		return nil, fmt.Errorf("no output directories provided")
	}

	log := clog.FromContext(ctx)
	log.Infof("Publishing config %s to %s", config.Name, repoRef)

	// Get platform from config
	platform, err := config.GetPlatform()
	if err != nil {
		return nil, fmt.Errorf("invalid platform in config: %w", err)
	}

	// Get disk formats from config (required by validation)
	configFormats, err := config.GetDiskFormats()
	if err != nil {
		return nil, fmt.Errorf("invalid disk formats in config: %w", err)
	}

	// Load artifacts for each architecture
	artifactsByArch := make(map[types.Architecture]*Artifacts)
	for arch, outputDir := range outputDirs {
		log.Infof("Loading artifacts for %s from %s", arch.ToAPK(), outputDir)

		artifacts, err := LoadArtifactsFromDir(outputDir, arch, platform)
		if err != nil {
			return nil, fmt.Errorf("loading artifacts for %s: %w", arch.ToAPK(), err)
		}

		// Apply disk format filtering from config
		// Build a set of formats to keep
		formatSet := make(map[DiskFormat]bool)
		for _, f := range configFormats {
			formatSet[f] = true
		}

		// Clear formats not specified in the config
		if !formatSet[DiskFormatRaw] {
			artifacts.DiskRaw = nil
		}
		if !formatSet[DiskFormatQcow2] {
			artifacts.DiskQcow2 = nil
		}
		if !formatSet[DiskFormatVmdk] {
			artifacts.DiskVmdk = nil
		}
		if !formatSet[DiskFormatVhd] {
			artifacts.DiskVhd = nil
		}
		if !formatSet[DiskFormatOva] {
			artifacts.DiskOva = nil
		}

		artifactsByArch[arch] = artifacts
		log.Infof("Loaded artifacts for %s: %v", arch.ToAPK(), artifacts.Summary())
	}

	// Parse the repository reference for operations
	ref, err := name.NewRepository(repoRef)
	if err != nil {
		return nil, fmt.Errorf("parsing repository reference: %w", err)
	}

	remoteOpts := append([]remote.Option{remote.WithContext(ctx)}, p.remoteOpts...)

	// Expand tags from config into timestamped and latest variants
	tags := ExpandTags(config.OCIConfig.Tags, opts.Timestamp)

	// Determine whether to use merge mode or batch mode
	var digest string

	if opts.Merge {
		// Merge mode: Try to fetch existing index and merge new architectures into it
		log.Infof("Merge mode enabled - checking for existing multi-arch index")

		// Use the first tag with "-latest" suffix to check for existing index
		// This ensures we're always merging into the current latest version
		var checkTag string
		for _, tag := range tags {
			if len(tag) >= 7 && tag[len(tag)-7:] == "-latest" {
				checkTag = tag
				break
			}
		}

		if checkTag == "" {
			return nil, fmt.Errorf("merge mode requires at least one tag ending with '-latest'")
		}

		log.Infof("Checking for existing index at tag: %s", checkTag)
		existingIdx, err := p.FetchExistingIndex(ctx, repoRef, checkTag, remoteOpts)
		if err != nil {
			return nil, fmt.Errorf("fetching existing index for merge: %w", err)
		}

		if existingIdx != nil {
			// Existing index found - merge new architectures into it
			log.Infof("Found existing index - merging new architectures")

			// Convert artifactsByArch to []ImageWithMetadata for merging
			var allImages []ImageWithMetadata
			for arch, artifacts := range artifactsByArch {
				log.Infof("Building images for %s", arch.ToAPK())

				if err := artifacts.Validate(); err != nil {
					return nil, fmt.Errorf("validating artifacts for %s: %w", arch.ToAPK(), err)
				}

				images, err := p.createImages(ctx, artifacts)
				if err != nil {
					return nil, fmt.Errorf("creating images for %s: %w", arch.ToAPK(), err)
				}
				allImages = append(allImages, images...)
			}

			// Merge and publish
			digest, err = p.MergeArchIntoIndex(ctx, existingIdx, allImages, ref, tags, opts.SkipIfExists, remoteOpts)
			if err != nil {
				return nil, fmt.Errorf("merging architectures into existing index: %w", err)
			}
			log.Infof("Successfully merged architectures into existing index: %s", digest)
		} else {
			// No existing index - fall back to batch mode
			log.Infof("No existing index found - creating new multi-arch index")
			digest, err = p.publishMultiArch(ctx, artifactsByArch, repoRef, opts)
			if err != nil {
				return nil, fmt.Errorf("publishing multi-arch index: %w", err)
			}
			log.Infof("Published new multi-arch index: %s", digest)

			// Apply tags since publishMultiArch doesn't do it
			digestRef, err := name.NewDigest(digest)
			if err != nil {
				return nil, fmt.Errorf("parsing digest: %w", err)
			}

			idx, err := remote.Index(digestRef, remoteOpts...)
			if err != nil {
				return nil, fmt.Errorf("fetching published index: %w", err)
			}

			log.Infof("Applying %d tags: %v", len(tags), tags)
			appliedTags, err := applyTagsToTarget(ctx, ref, idx, tags, remoteOpts)
			if err != nil {
				return nil, fmt.Errorf("applying tags: %w", err)
			}

			log.Infof("Publishing complete. Applied %d tags", len(appliedTags))
			return appliedTags, nil
		}
	} else {
		// Batch mode: Create fresh multi-arch index with all provided architectures
		log.Infof("Batch mode - creating new multi-arch index for %s", config.Name)
		digest, err = p.publishMultiArch(ctx, artifactsByArch, repoRef, opts)
		if err != nil {
			return nil, fmt.Errorf("publishing multi-arch index: %w", err)
		}
		log.Infof("Published multi-arch index: %s", digest)

		// Fetch the published index for tagging
		digestRef, err := name.NewDigest(digest)
		if err != nil {
			return nil, fmt.Errorf("parsing digest: %w", err)
		}

		idx, err := remote.Index(digestRef, remoteOpts...)
		if err != nil {
			return nil, fmt.Errorf("fetching published index: %w", err)
		}

		// Apply tags
		log.Infof("Applying %d tags: %v", len(tags), tags)
		appliedTags, err := applyTagsToTarget(ctx, ref, idx, tags, remoteOpts)
		if err != nil {
			return nil, fmt.Errorf("applying tags: %w", err)
		}

		log.Infof("Publishing complete. Applied %d tags", len(appliedTags))
		return appliedTags, nil
	}

	// If we got here via merge mode with existing index, tags were already applied by MergeArchIntoIndex
	log.Infof("Publishing complete via merge mode")
	return tags, nil
}
