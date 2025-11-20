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

	// Config-driven flags
	var configPath string
	var outputDir string
	var registry string
	var architecture string

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish VM artifacts to an OCI registry",
		Long: `Publish VM disk images and related artifacts to an OCI registry using a publish.yaml config.

Config-driven publishing uses publish.yaml files to define OCI image names, tags, and disk formats.

Each architecture is published separately and automatically merged into the existing multi-arch
index. This allows parallel CI/CD builds where different runners can publish different
architectures concurrently.

USAGE:
  Publish single architecture (merges into existing index or creates new one):
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/x86_64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architecture x86_64 \
      --timestamp 20251103-1234

  Publish another architecture (automatically merges with existing):
    apkoaas publish \
      --config configs/azure-python-313-slim/publish.yaml \
      --output-dir output/aarch64/azure-python-313-slim \
      --registry cgr.dev/chainguard-vms \
      --architecture aarch64 \
      --timestamp 20251103-1234

CONCURRENT PUBLISHING:
  The system automatically handles concurrent publishes from parallel CI/CD runners:
  - Each runner publishes its architecture independently
  - Before applying tags, the system checks for concurrent publishes
  - If another architecture was published concurrently, indexes are merged
  - Final result contains all architectures from all concurrent publishes

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

Each multi-arch index contains sub-indexes for each architecture with all artifact types
(apko tar, disk formats, SBOMs, secure boot files).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Validate required flags
			if configPath == "" {
				return fmt.Errorf("--config is required")
			}

			return publishFromConfig(ctx, configPath, outputDir, registry, architecture, timestamp, skipIfExists, signAndAttest, attestationKey, skipTransparencyLog, userAgent)
		},
	}

	// Config-driven publishing flags (required)
	cmd.Flags().StringVar(&configPath, "config", "", "Path to publish.yaml config file (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory containing artifacts (required)")
	cmd.Flags().StringVar(&registry, "registry", "", "Registry prefix like 'cgr.dev/chainguard-vms' (required)")
	cmd.Flags().StringVar(&architecture, "architecture", "x86_64", "Architecture to publish (x86_64 or aarch64)")

	// Publishing options
	cmd.Flags().StringVar(&timestamp, "timestamp", "", "Timestamp for tag expansion (format: YYYYMMDD-HHMM, default: auto-generated)")

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
func publishFromConfig(ctx context.Context, configPath, outputDir, registry, architecture, timestamp string, skipIfExists, signAndAttest bool, attestationKey string, skipTransparencyLog bool, userAgent string) error {
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

	// Build output directory map for the single architecture
	arch := types.ParseArchitecture(architecture)
	outputDirs := map[types.Architecture]string{
		arch: outputDir,
	}
	slog.InfoContext(ctx, "Publishing architecture", "arch", arch.ToAPK(), "dir", outputDir)

	// Configure remote options
	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
	if userAgent != "" {
		remoteOpts = append(remoteOpts, remote.WithUserAgent(userAgent))
	}

	// Create publisher
	pub := publisher.New(remoteOpts...)

	// Publish using config-driven approach with incremental merging
	opts := &publisher.PublishOptions{
		SkipIfExists:        skipIfExists,
		SignAndAttest:       signAndAttest,
		AttestationKeyRef:   attestationKey,
		SkipTransparencyLog: skipTransparencyLog,
		Timestamp:           timestamp,
	}

	appliedTags, err := pub.PublishSingleConfig(ctx, config, outputDirs, repoRef, opts)
	if err != nil {
		return fmt.Errorf("publishing from config: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Published config %s to: %s\n", config.Name, repoRef)
	fmt.Fprintf(os.Stdout, "Applied tags: %v\n", appliedTags)
	return nil
}


