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
	"chainguard.dev/apko/pkg/build/types"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrtypes "github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	// MediaTypeVMRootFS uses standard OCI layer media type for compatibility with scanners
	MediaTypeVMRootFS = ggcrtypes.OCILayer // "application/vnd.oci.image.layer.v1.tar+gzip"

	// MediaTypeOCIEmpty is used for OCI artifact configs (not container images)
	// per https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidelines-for-artifact-usage
	MediaTypeOCIEmpty = "application/vnd.oci.empty.v1+json"

	// The APKO produced SBOM is an SPDX json file
	MediaTypeVMApkoSbom = "application/spdx+json"

	// Custom media types for VM disk artifacts
	MediaTypeVMDiskRaw    = "application/vnd.chainguard.vm.disk.raw.tgz.v1+gzip"
	MediaTypeVMDiskQcow2  = "application/vnd.chainguard.vm.disk.qcow2.v1+gzip"
	MediaTypeVMDiskVmdk   = "application/vnd.chainguard.vm.disk.vmdk.v1+gzip"
	MediaTypeVMDiskVhd    = "application/vnd.chainguard.vm.disk.vhd.v1+gzip"
	MediaTypeVMDiskOva    = "application/vnd.chainguard.vm.disk.ova.v1+gzip"
	MediaTypeVMSecureBoot = "application/vnd.chainguard.vm.secureboot.v1+gzip"
	MediaTypeVMSyftSbom   = "application/vnd.chainguard.vm.syft.sbom.json.v1+gzip"

	AnnotationBuildTime    = "org.opencontainers.image.created"
	AnnotationArchitecture = "org.opencontainers.image.architecture"
	AnnotationArtifactType = "org.chainguard.vm.artifact.type"
)

// ImageWithMetadata represents an OCI image with its associated metadata
type ImageWithMetadata struct {
	Image        v1.Image
	Arch         types.Architecture
	ArtifactType string // e.g., "apko.v1", "disk.raw.tgz.v1", "disk.qcow2.v1", "sbom.v1", "secureboot.v1"
}

// PublishOptions contains optional parameters for publishing VM artifacts
type PublishOptions struct {
	SkipIfExists        bool   // If true, check if images exist and skip push if they do
	SignAndAttest       bool   // If true, sign and attach attestations after publishing
	AttestationKeyRef   string // Path to private key for signing (empty = keyless/OIDC)
	SkipTransparencyLog bool   // If true, skip logging to Rekor transparency log
	Timestamp           string // Timestamp for cloud-specific tags (e.g., "20241103-1234")
}
