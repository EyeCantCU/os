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

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"chainguard.dev/apko/pkg/build/types"
	"chainguard.dev/wolfi-vm/pkg/publisher"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

func publishCmd() *cobra.Command {
	var userAgent string
	var skipIfExists bool
	var signAndAttest bool
	var attestationKey string
	var skipTransparencyLog bool
	var timestamp string
	var merge bool

	// Config-driven flags
	var configPath string
	var outputDir string
	var registry string
	var architectures []string

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish VM artifacts to an OCI registry",
		Long: `Publish VM disk images and related artifacts to an OCI registry using a publish.yaml config.

Config-driven publishing uses publish.yaml files to define OCI image names, tags, and disk formats.

USAGE:
  Basic publishing with single architecture:
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/x86_64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architectures x86_64

  Multi-architecture publishing (batch mode - default):
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/x86_64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architectures x86_64,aarch64 \
      --timestamp 20251103-1234

  Incremental publishing (merge mode):
    # First architecture (creates new index)
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/x86_64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architectures x86_64 \
      --timestamp 20251103-1234 \
      --merge

    # Second architecture (merges into existing index)
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/aarch64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architectures aarch64 \
      --timestamp 20251103-1234 \
      --merge

PUBLISH CONFIG FORMAT (publish.yaml):
  version: 1
  cloud: azure
  name: azure-python-313-slim
  oci_config:
    image: python                   # Combined with --registry to form full repo
    tags:
      - azure-python-3.13-slim      # Base tags (each expands to 2 variants)
      - azure-python-slim
    disk_formats:                   # Required: specify which formats to publish
      - vhd
      - vmdk

TAG EXPANSION:
  Each base tag is automatically expanded into 2 variants:
    - tag-TIMESTAMP (e.g., azure-python-slim-20251103-1234)
    - tag-latest (e.g., azure-python-slim-latest)

  Timestamp is auto-generated or specified with --timestamp flag.

PUBLISHED STRUCTURE:
  Repository: cgr.dev/chainguard-vms/python
  Tags (2 variants per base tag):
    azure-python-3.13-slim-20251103-1234  → multi-arch index
    azure-python-3.13-slim-latest         → multi-arch index
    azure-python-slim-20251103-1234       → multi-arch index
    azure-python-slim-latest              → multi-arch index

Each multi-arch index contains x86_64 and aarch64 sub-indexes with all artifact types
(apko tar, disk formats, SBOMs, secure boot files).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate required flags
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}

			return publishFromConfig(ctx, configPath, outputDir, registry, architectures, timestamp, skipIfExists, signAndAttest, attestationKey, skipTransparencyLog, merge, userAgent)
		},
	}

	// Config-driven publishing flags (required)
	cmd.Flags().StringVar(&configPath, "config", "", "Path to publish.yaml config file (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory containing artifacts (required)")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry prefix like 'cgr.dev/chainguard-vms' (required)")
	cmd.Flags().StringSliceVar(&architectures, "architectures", []string{"x86_64", "aarch64"}, "Architectures to publish")

	// Publishing options
	cmd.Flags().StringVar(&timestamp, "timestamp", "", "Timestamp for tag expansion (format: YYYYMMDD-HHMM, default: auto-generated)")
	cmd.Flags().BoolVar(&merge, "merge", false, "Merge new architectures into existing multi-arch index (incremental publishing)")

	// Registry options
	cmd.Flags().StringVar(&userAgent, "user-agent", "wolfi-vm-publisher", "User agent for registry requests")
	cmd.Flags().BoolVar(&skipIfExists, "skip-if-exists", true, "Skip publishing if the image already exists in the registry")

	// Signing and attestation options
	cmd.Flags().BoolVar(&signAndAttest, "sign-and-attest", true, "Sign images and attach attestations using cosign")
	cmd.Flags().StringVar(&attestationKey, "attestation-key", "", "Path to private key for signing (empty = keyless/OIDC)")
	cmd.Flags().BoolVar(&skipTransparencyLog, "skip-transparency-log", false, "Skip logging to Rekor transparency log")

	return cmd
}

// publishFromConfig handles config-driven publishing using a publish.yaml file
func publishFromConfig(ctx context.Context, configPath, outputDir, registry string, architectures []string, timestamp string, skipIfExists, signAndAttest bool, attestationKey string, skipTransparencyLog, merge bool, userAgent string) error {
	// Validate required flags
	if outputDir == "" {
		return fmt.Errorf("--output-dir is required when using --config")
	}
	if registry == "" {
		return fmt.Errorf("--registry is required when using --config")
	}

	// Auto-generate timestamp if not provided
	if timestamp == "" {
		timestamp = time.Now().Format("20060102-1504")
		slog.InfoContext(ctx, "Auto-generated timestamp", "timestamp", timestamp)
	}

	// Load the publish config
	slog.InfoContext(ctx, "Loading publish config", "path", configPath)
	config, err := publisher.LoadPublishConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading publish config: %w", err)
	}

	// Construct full repository reference
	repoRef := fmt.Sprintf("%s/%s", registry, config.OCIConfig.Image)
	slog.InfoContext(ctx, "Publishing config", "name", config.Name, "cloud", config.Cloud, "repository", repoRef)

	// Build output directory map for each architecture
	//
	// For batch mode (multi-arch publishing), the user should provide an outputDir that:
	// 1. Contains one architecture's path (e.g., output/x86_64/config-name)
	// 2. Allows us to derive other architectures by replacing the arch component
	//
	// For incremental mode (--merge), the user provides a single architecture's outputDir.
	//
	// Expected directory structure:
	//   output/x86_64/azure-python-313-slim/
	//   output/aarch64/azure-python-313-slim/
	outputDirs := make(map[types.Architecture]string)

	if len(architectures) == 1 {
		// Single architecture - use outputDir directly (works for both merge and batch modes)
		arch := types.ParseArchitecture(architectures[0])
		outputDirs[arch] = outputDir
		slog.InfoContext(ctx, "Publishing single architecture", "arch", arch.ToAPK(), "dir", outputDir)
	} else {
		// Multiple architectures (batch mode) - derive paths by replacing architecture component
		// Try to detect and replace the architecture string in the path
		slog.InfoContext(ctx, "Batch mode: deriving paths for multiple architectures", "archs", architectures, "baseDir", outputDir)

		for _, archStr := range architectures {
			arch := types.ParseArchitecture(archStr)

			// Try to derive the path by replacing known architecture strings
			derivedPath := outputDir
			replaced := false

			// Try replacing x86_64 or aarch64 in the path
			for _, replaceArch := range []string{"x86_64", "aarch64"} {
				if replaceArch != arch.ToAPK() {
					// Found a different architecture in the path - replace it
					newPath := outputDir
					// Simple string replacement
					for i := range outputDir {
						if i+len(replaceArch) <= len(outputDir) {
							if outputDir[i:i+len(replaceArch)] == replaceArch {
								newPath = outputDir[:i] + arch.ToAPK() + outputDir[i+len(replaceArch):]
								replaced = true
								break
							}
						}
					}
					if replaced {
						derivedPath = newPath
						break
					}
				}
			}

			// If we couldn't derive the path, check if the current path contains this arch
			if !replaced {
				// Check if outputDir contains the current arch string
				containsArch := false
				for i := range outputDir {
					if i+len(arch.ToAPK()) <= len(outputDir) {
						if outputDir[i:i+len(arch.ToAPK())] == arch.ToAPK() {
							containsArch = true
							break
						}
					}
				}

				if !containsArch {
					return fmt.Errorf("cannot derive output path for %s: outputDir %q does not contain an architecture component (x86_64 or aarch64) that can be replaced", arch.ToAPK(), outputDir)
				}
			}

			outputDirs[arch] = derivedPath
			slog.InfoContext(ctx, "Derived output directory for architecture", "arch", arch.ToAPK(), "dir", derivedPath)
		}
	}

	// Configure remote options
	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
	if userAgent != "" {
		remoteOpts = append(remoteOpts, remote.WithUserAgent(userAgent))
	}

	// Create publisher
	pub := publisher.New(remoteOpts...)

	// Publish using config-driven approach
	opts := &publisher.PublishOptions{
		SkipIfExists:        skipIfExists,
		SignAndAttest:       signAndAttest,
		AttestationKeyRef:   attestationKey,
		SkipTransparencyLog: skipTransparencyLog,
		Timestamp:           timestamp,
		ArtifactTags:        false, // Config-driven doesn't use artifact tags for now
		Merge:               merge,
	}

	appliedTags, err := pub.PublishSingleConfig(ctx, config, outputDirs, repoRef, opts)
	if err != nil {
		return fmt.Errorf("publishing from config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Published config %s to: %s\n", config.Name, repoRef)
	fmt.Fprintf(os.Stdout, "Applied tags: %v\n", appliedTags)
	return nil
}


