# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `wolfi-vm`, a project for building virtual machine disk images from apko YAML configurations. It's similar to container building but creates bootable VM images for various cloud platforms (AWS, Azure, GCP) and local QEMU testing.

## Key Components

- **Main CLI**: `apkoaas` binary built from Go code in `main.go` and `pkg/cli/`
- **Build System**: Uses both Makefile and Go-based CLI for VM image creation
- **Image Configs**: YAML files in `configs/` directory define packages and settings for different VM variants
- **Cloud Support**: Platform-specific configurations for AWS, Azure, GCP, and QEMU images
- **Testing Framework**: Comprehensive testing system in `vms-test/` for validating images across platforms

## Architecture

The project follows this workflow:
1. **Config Definition**: YAML configs in `configs/` specify packages, repositories, and settings
2. **Build Process**: `apkoaas build` command converts configs to raw disk images using apko
3. **Cloud Publishing**: Platform-specific tools upload and register images in cloud platforms
4. **Testing**: Automated test runners validate functionality across different environments

Core directories:
- `configs/`: VM configuration files (build.yaml, test.yaml per variant)
- `pkg/`: Go source code (CLI, converter, utilities)
- `tools/`: Shell scripts for cloud operations and image manipulation
- `vms-test/`: Testing framework with cloud-specific runners
- `awspub/`, `Makefile`: Cloud-specific publishing configurations

## Common Commands

### Building Images
```bash
# Build specific VM image
make disk-<config-name>
# Examples:
make disk-qemu-base-slim
make disk-aws-base-slim
make disk-azure-docker-slim

# Build all images for a cloud platform
make disks-aws
make disks-azure
make disks-gcp
make disks-qemu

# Build debug version with shell access
make disk-debug-<config-name>
```

### Running/Testing Images
```bash
# Run VM locally with QEMU
make run-<config-name>
make run-qemu-base-slim  # Boot locally, SSH via port 6379

# Run debug version with console access
make run-debug-<config-name>
make debug-shell-<config-name>  # Connect to debug console

# Run comprehensive tests
make test-gotest            # Go unit tests
make test-qemu-base-slim         # Full VM testing on qemu-base-slim image
```

### Development
```bash
# Build the CLI tool
make apkoaas
go build -o apkoaas

# Install system dependencies (Ubuntu/Debian)
make install-deps

# Clean build artifacts
make clean
```

### Publishing

The project uses a **multi-cloud** publishing approach where all cloud variants of an application are published to a single OCI repository with cloud-specific tags. Each cloud variant is a complete, separate multi-arch image with its own kernel, packages, and disk formats.

#### Multi-Cloud Structure

```
Repository: cgr.dev/chainguard-vms/base

Tags (each pointing to a separate multi-arch index):
  qemu-latest               → qemu multi-arch index
  qemu-20241103-1234        → qemu timestamped
  aws-latest                → aws multi-arch index (AWS kernel + packages)
  aws-20241103-1234         → aws timestamped
  azure-latest              → azure multi-arch index (Azure kernel + packages)
  azure-20241103-1234       → azure timestamped
  gcp-latest                → gcp multi-arch index (GCP kernel + packages)
  gcp-20241103-1234         → gcp timestamped

Each multi-arch index contains:
  - x86_64 sub-index → apko tar, disk.raw.tgz, disk.qcow2, disk.vmdk, disk.vhd, SBOMs
  - aarch64 sub-index → apko tar, disk.raw.tgz, disk.qcow2, disk.vmdk, disk.vhd, SBOMs
```

#### Publishing Commands

Each architecture is published separately and automatically merged into the existing multi-arch
index. This allows parallel CI/CD builds where different runners can publish different
architectures concurrently.

**Key Features:**
- Each architecture is published independently
- New architectures automatically merge into existing multi-arch OCI index
- Safe for CI/CD pipelines with parallel arch-specific runners
- Creates new index if none exists
- Automatically detects and handles concurrent publishes

**Makefile Targets:**
```bash
# Publish current architecture (default: x86_64)
make publish-registry-azure-python-313-slim
make publish-registry-qemu-base-slim
make publish-registry-aws-base-slim

# Publish different architecture
ARCH=aarch64 make publish-registry-azure-docker-full

# Publish with custom registry and timestamp
REGISTRY_REPO=cgr.dev/custom-org BUILD_TIMESTAMP=20250115-1200 make publish-registry-gcp-nginx-full

# Parallel CI/CD workflow example:
# Runner 1: ARCH=x86_64 make publish-registry-aws-base-slim
# Runner 2: ARCH=aarch64 make publish-registry-aws-base-slim
# Result: Multi-arch index with both x86_64 and aarch64
```

**Direct CLI Publishing:**
```bash
# First architecture (creates new index or merges if exists)
./apkoaas publish \
  --config configs/azure-python-313-slim/publish.yaml \
  --output-dir output/x86_64/azure-python-313-slim \
  --registry cgr.dev/chainguard-vms \
  --architecture x86_64 \
  --timestamp 20251103-1234

# Second architecture (automatically merges into existing index)
./apkoaas publish \
  --config configs/azure-python-313-slim/publish.yaml \
  --output-dir output/aarch64/azure-python-313-slim \
  --registry cgr.dev/chainguard-vms \
  --architecture aarch64 \
  --timestamp 20251103-1234
```

**Cloud-Specific Publishing (AWS, Azure, GCP):**
```bash
# Publish to cloud-specific registries (creates AMIs, Azure images, GCP images)
make publish-azure-<config-name>
make publish-gcp-<config-name>
make publish-qemu-<config-name>

# AWS publishing (uses awspub configuration)
make aws-create-<image-name>
make aws-publish-<image-name>

# Examples with environment targeting:
PUBLISH_TARGET=dev make publish-gcp-base-slim
PUBLISH_TARGET=staging make publish-azure-docker-slim
PUBLISH_TARGET=eap make publish-qemu-base-slim
```

#### Publishing Variables

**OCI Registry Publishing:**
- `REGISTRY_REPO`: Base repository path (default: `cgr.dev/chainguard-vms`)
- `BUILD_TIMESTAMP`: Timestamp for version tags (auto-generated as `YYYYMMDD-HHMM`)
- `ARCH`: Target architecture (x86_64 or aarch64)

**Cloud-Specific Publishing:**
- `PUBLISH_TARGET`: Publishing environment (dev/staging/eap/production) - for cloud-specific publishing

#### Race Condition Handling

The publishing system includes automatic protection against race conditions when multiple CI/CD runners publish different architectures concurrently.

**How It Works:**

1. **Initial Publish**: Each runner publishes its architecture-specific images and creates an index
2. **Pre-Tag Re-check**: Before applying tags, the system re-checks the registry for concurrent publishes
3. **Automatic Merging**: If another index was published while we were building, both indexes are merged
4. **Final Result**: Tags are applied to the merged index containing all architectures

**Implementation Details:**

The race condition handling is implemented in `pkg/publisher/publisher.go`:
- `recheckAndMergeIfNeeded()`: Re-checks registry and initiates merge if needed
- `mergePublishedIndexes()` (in `pkg/publisher/index.go`): Fetches both indexes, combines sub-index manifests, publishes merged result
- Always enabled for all publishes
- Uses the `-latest` tag as the check point for detecting concurrent publishes

**Why This Matters:**

Without this protection:
- CI Runner 1 publishes x86_64 → Runner 2 publishes aarch64 → Runner 2's tags overwrite → Only aarch64 remains

With this protection:
- CI Runner 1 publishes x86_64 → Runner 2 publishes aarch64 → Runner 2 detects and merges → Final index has both architectures

**Edge Cases Handled:**

- Both runners finish at exactly the same time: Last to tag wins, but merges the other's work
- Runner 1 fails after publishing: Runner 2 detects and includes Runner 1's index in merge
- No concurrent publish detected: Original index is tagged without modification

#### Platform Validation and Error Handling

**Strict Platform Detection**: The publisher package enforces strict platform validation with no fallback behavior. All platforms must be explicitly recognized.

**Valid Platforms:**
- `aws` - Amazon Web Services
- `azure` - Microsoft Azure
- `gcp` - Google Cloud Platform
- `vmware` - VMware
- `qemu` - QEMU images
- `rpi` - Raspberry Pi

**Error-Returning Functions:**
The following functions return errors for unrecognized platforms (no silent fallbacks):
- `ParsePlatform(s string) (Platform, error)` - Parse platform string like "aws", "azure"
- `ToCloudPlatforms() ([]string, error)` - Convert platform to cloud identifiers

**Failure Modes:**
- Unknown platform strings cause immediate errors: `unrecognized platform: "foo"`
- Invalid directory naming (no hyphen) fails: `no hyphen found in basename "test"`
- Invalid enum values fail: `unrecognized platform enum value: unknown`

#### Troubleshooting Publishing Errors

**Error: `unrecognized platform: "foo"`**
- **Cause**: Directory name doesn't start with a valid platform prefix
- **Solution**: Rename directory to use one of: `aws`, `azure`, `gcp`, `vmware`, `qemu`, `rpi`
- **Example**: Rename `foo-base` → `qemu-base` or `aws-base`

**Error: `no hyphen found in basename "myapp"`**
- **Cause**: Directory name missing hyphen separator between platform and application
- **Solution**: Add hyphen in format `{platform}-{application}`
- **Example**: Rename `awsbase` → `aws-base`, `test` → `qemu-test`

**Error: `failed to detect platform from output directory`**
- **Cause**: Output directory path doesn't contain a recognizable platform-prefixed directory
- **Solution**: Ensure output structure is `output/{arch}/{platform}-{app}/`
- **Example**: Check that you have `output/x86_64/aws-base/` not `output/x86_64/base/`

**Error: `unrecognized platform enum value`**
- **Cause**: Internal error - corrupted or invalid Platform value in code
- **Solution**: This shouldn't happen in normal use. Check for manual Platform type construction without validation
- **Prevention**: Always use `ParsePlatform()` to create Platform values

#### Direct Publishing with apkoaas

For advanced use cases or scripting, you can call `apkoaas publish` directly.

**Config-Driven Publishing:**
Publishing requires a `publish.yaml` config file that defines the OCI image name and tags:

```bash
# Basic single-architecture publishing
./apkoaas publish \
  --config configs/azure-python-313-slim/publish.yaml \
  --output-dir output/x86_64/azure-python-313-slim \
  --registry cgr.dev/chainguard-vms \
  --architectures x86_64

# Multi-architecture publishing with timestamp
./apkoaas publish \
  --config configs/azure-python-313-slim/publish.yaml \
  --output-dir output/x86_64/azure-python-313-slim \
  --registry cgr.dev/chainguard-vms \
  --architectures x86_64,aarch64 \
  --timestamp 20241103-1234
```

**The publish.yaml config specifies:**
- `image`: The OCI image name (e.g., `python` becomes `cgr.dev/chainguard-vms/python`)
- `tags`: List of base tag names (each expands to 2 variants: `tag-TIMESTAMP` and `tag-latest`)
- `disk_formats`: **Required** list specifying which disk formats to publish (e.g., `[vhd]`, `[qcow2]`, `[raw.tgz, vmdk]`)

**Example publish.yaml:**
```yaml
version: 1
cloud: azure
name: azure-python-313-slim
oci_config:
  image: python
  tags:
    - azure-python-3.13-slim    # Expands to -20241103-1234 and -latest variants
    - azure-python-slim
  disk_formats:  # Required: specify which formats to publish
    - vhd
```

**Tag Expansion:**
Each tag is automatically expanded into 2 variants:
- `tag-TIMESTAMP`: e.g., `azure-python-slim-20241103-1234`
- `tag-latest`: e.g., `azure-python-slim-latest`

**Disk Format Selection:**
The `disk_formats` field is required and explicitly controls which disk formats appear in the published OCI artifacts:
- Specify exactly which formats you want (e.g., `[vhd]` for Azure, `[qcow2]` for QEMU)
- All specified formats must exist in the output directory
- Common combinations:
  - AWS: `[vmdk]` - for EC2 AMI import
  - Azure: `[vhd]` - required for Azure VMs
  - GCP: `[raw]` - for Compute Engine
  - QEMU: `[qcow2]` - for local testing
  - VMware: `[vmdk, ova]` - VMware formats
  - Multi-cloud: `[raw, qcow2, vmdk, vhd]` - publish multiple formats

#### Cloud-Specific Publishing (AWS, Azure, GCP)

For publishing to cloud-specific image registries (not OCI registries):

```bash
# Publish to cloud-specific registries (creates AMIs, Azure images, GCP images)
make publish-azure-<config-name>
make publish-gcp-<config-name>
make publish-qemu-<config-name>

# AWS publishing (uses awspub configuration)
make aws-create-<config-name>
make aws-publish-<config-name>

# With environment targeting
PUBLISH_TARGET=staging make publish-azure-base
PUBLISH_TARGET=eap make publish-gcp-docker

# Output tracking files:
# - Azure: output/<arch>/<image>/publish.<env>.json
# - GCP: output/<arch>/<image>/publish.<env>.yaml
# - QEMU: output/<arch>/<image>/publish.<env>.json
# - AWS: output/<arch>/awspub/publish/<image>.output
```

## Configuration Structure

Each config in `configs/` contains:
- `build.yaml`: Apko configuration defining packages, repositories, architecture
- `test.yaml`: Test specifications for validation

Example config structure:
```
configs/aws-base-slim/
├── build.yaml    # Package list, repos, arch settings
└── test.yaml     # Test cases for this image variant
```

**Config Organization (REQUIRED):**
- **Naming Pattern**: All configs MUST follow `{cloud}-{application}[-variant]` pattern
- **Valid Cloud Prefixes**: `aws`, `azure`, `gcp`, `vmware`, `qemu`, `rpi`
- **Examples**:
  - ✅ Valid: `aws-base`, `azure-docker-dev`, `qemu-nginx-full`, `gcp-eks-1.33`
  - ❌ Invalid: `base` (no cloud prefix), `unknown-base` (invalid cloud), `myapp` (no hyphen)

**Why This Matters:**
- Publisher extracts platform from directory names for validation
- Invalid naming causes publishing to fail with clear error messages
- Platform is used for validation and organization (disk formats are specified in publish.yaml)

**Multi-Cloud Applications:**
- Applications can have multiple cloud variants (e.g., `aws-base`, `azure-base`, `gcp-base`)
- The Makefile provides per-application grouping via `app_*` variables and `disks-app-*` targets
- Single-cloud applications (like `aws-eks-*`) also get per-application organization

## Testing

The project includes extensive testing infrastructure:
- **Unit Tests**: `go test ./...` for Go code
- **VM Integration Tests**: Full VM boot and functionality testing
- **Cloud Platform Tests**: Test runners for AWS, Azure, GCP in `vms-test/`

### VM Integration Testing (`vms-test/`)

The `vms-test/` directory contains a comprehensive testing framework for validating VM images across different cloud platforms and QEMU. The testing system follows a structured approach with test helpers, cloud-specific runners, and detailed result collection.

#### Test Architecture

**Core Components:**
- **Test Helpers**: Scripts in `vms-test/helpers/` that orchestrate testing workflows
- **Cloud Runners**: Platform-specific VM management tools in `vms-test/pkg/runner/`
- **Test Groups**: Organized test suites in `vms-test/pkg/test/` covering different functionality areas
- **Metrics Collection**: Structured logging and performance data collection
- **Configuration Files**: YAML configs in `configs/test/` defining test parameters per platform

#### Test Groups and Structure

Tests are organized into logical groups under `vms-test/pkg/test/`:

- **core/**: Basic VM functionality (SSH, systemd, kernel)
- **containers/**: Container runtime testing (Docker)
- **applications/**: Application-specific tests (QEMU)
- **aws/**: AWS-specific functionality (SSM, EC2 features)
- **azure/**: Azure-specific features (resource disk, routing)
- **gcp/**: GCP-specific features (startup scripts, routing)
- **hypervisor/**: Virtualization features (nested virtualization)

#### Running Integration Tests

**Basic Test Execution:**
```bash
# Run comprehensive VM testing on qemu-base-slim image
make test-qemu-base-slim

# Build and run tests for specific architecture
make -C vms-test TEST_ARCHES="x86_64" runners tests

# Run tests using helpers (manual execution)
QEMU_VMS="qemu-base-slim" ./vms-test/helpers/test-wolfi-vm --wolfi-vm $PWD qemu ./test-results
./vms-test/helpers/test-wolfi-vm --wolfi-vm $PWD aws ./test-results -- --launch-group=some-launch-group --awspub-inputs=./output/$arch/awspub/create/
AZ_SUBSCRIPTION=$(az account show | jq -r .id) ./vms-test/helpers/test-wolfi-vm --wolfi-vm $PWD azure ./test-results -- --subscription-id $AZ_SUBSCRIPTION --resource-group some-resource-group --azure-inputs output/
GCP_PROJECT=$(gcloud config get project) ./vms-test/helpers/test-wolfi-vm --wolfi-vm $PWD gcp ./test-results -- --project $GCP_PROJECT --gcp-inputs ./output/
```

#### Test Configuration Files

Each cloud platform has test configuration files in `configs/test/<cloud>/`:

```yaml
# Example: configs/test/aws/base.yaml
version: 1
cloud: aws
data:
  vmuser: ec2-user
vmconfigs:
  x86_64:
    - instance_type: t3.medium
      tests:
        - group: core
  aarch64:
    - instance_type: t4g.medium
      tests:
        - group: core
```

#### Image Selection and Publishing Integration

The test helpers automatically discover and test images using publishing output files:

**AWS Image Selection:**
- Uses `awspub create` JSON output files from `output/<arch>/awspub/create/`
- Parses AMI IDs and regions from JSON metadata
- Example input: `output/x86_64/awspub/create/aws-base-slim.json`

**Azure Image Selection:**
- Uses published image information from Azure galleries
- Supports latest versions or specific image versions
- Configured via gallery name, resource group, and image name parameters

**GCP Image Selection:**
- Uses `publish.<env>.yaml` files from `output/<arch>/gcp-*/publish.<env>.yaml`
- Extracts image URIs and metadata from YAML output
- Supports filtering by project and image family

**QEMU Testing:**
- Uses local disk images from `output/<arch>/qemu-*/disk.raw`
- Supports both raw and qcow2 formats

#### Metrics and Logging

The testing framework includes comprehensive metrics collection and file artifact storage:

**Metrics Collection:**
- Performance metrics (boot time, service start times)
- Test execution results and timing
- Resource utilization data
- Custom application metrics

**Artifact Logging:**
Metrics and files are stored by their unique IDs defined in `vms-test/pkg/artifacts/{metrics,files}/*.go`:

```go
// Example metric logging
artifacts.Log(t, metrics.SSHDStartTime, fmt.Sprintf("%f", st.Seconds()), nil)

// Example file logging (files are base64 encoded)
artifacts.File(t, files.Dmesg, dmesgContent, nil, nil)
```

Files are automatically base64 encoded when stored. See the generated ID files for available metric and file IDs.

**Output Structure:**
```
test-results/<arch>/
├── <cloud>-<image>-<instance-type>/
│   ├── console.log                          # VM console output
│   ├── <test-group>-<test-name>-artifacts.json  # Test artifacts (metrics & files)
│   ├── test-<name>.stdout                   # Test stdout logs
│   ├── test-<name>.stderr                   # Test stderr logs
│   └── compliance-reports-artifacts.json   # Compliance test artifacts
```

**Artifacts Storage:**
- Each test creates a separate `-artifacts.json` file containing metrics and base64-encoded files
- Metrics are stored with their predefined IDs for structured access
- Test execution logs are stored as separate stdout/stderr files

#### Analyzing Test Results

**Analysis Priority: Always look for package-level root causes first. Search through test logs, console output, and system logs systematically to identify the underlying package or dependency that's causing failures.**

**Step-by-Step Analysis Workflow:**

1. **Start with test execution logs to identify which specific tests failed:**
   ```bash
   # Find all failed tests quickly
   find test-results/ -name "*.stdout" -o -name "*.stderr" | xargs grep -l "FAIL\|ERROR\|panic"

   # Get summary of all test failures
   find test-results/ -name "*test*.stdout" -exec grep -H "FAIL\|--- FAIL" {} \;
   ```

2. **Examine console logs for boot/system level issues:**
   ```bash
   # Look for critical system failures first
   find test-results/ -name "console.log" -exec grep -l "kernel panic\|segfault\|systemd.*failed\|emergency mode" {} \;

   # Check for package installation/dependency failures
   find test-results/ -name "console.log" -exec grep -A5 -B5 "apk.*failed\|package.*not found\|dependency.*failed" {} \;
   ```

3. **Extract and analyze system logs from artifacts for deeper investigation:**
   ```bash
   # Extract systemd journal to find service failures
   find test-results/ -name "*-artifacts.json" -exec jq -r '.Functions[].files[] | select(.ID=="JournalCtlB0") | .Content' {} \; | base64 -d > /tmp/journal.log

   # Extract dmesg for kernel/driver issues
   find test-results/ -name "*-artifacts.json" -exec jq -r '.Functions[].files[] | select(.ID=="Dmesg") | .Content' {} \; | base64 -d > /tmp/dmesg.log

   # Look for package-level failures in systemd logs
   grep -i "failed\|error\|panic" /tmp/journal.log | grep -v "DEBUG"
   ```

4. **Check performance metrics for timing-related failures:**
   ```bash
   # Extract key timing metrics that might indicate issues
   find test-results/ -name "*-artifacts.json" -exec jq -r '.Functions[].metrics[] | select(.ID=="SSHDStartTime" or .ID=="MultiUserTarget") | "\(.ID): \(.Value)s"' {} \;

   # Look for unusually long boot times (>60s is suspicious)
   find test-results/ -name "*-artifacts.json" -exec jq -r '.Functions[].metrics[] | select(.ID=="MultiUserTarget" and (.Value|tonumber) > 60) | "SLOW BOOT: \(.Value)s"' {} \;
   ```

5. **Cross-reference against build configurations to identify package issues:**
   ```bash
   # When you find a failure, check the corresponding build config
   # Look for recently changed packages or missing dependencies
   # Example: if aws-base-slim tests fail, examine configs/aws-base-slim/build.yaml
   ```

**Common Failure Patterns to Look For:**
- **Package dependency failures**: Search logs for "apk add failed", "package not found", or "dependency resolution"
- **Service startup failures**: Look for systemd service failures in journal logs
- **Kernel module issues**: Check dmesg for driver/module loading failures
- **Network configuration**: SSH failures often indicate networking or user setup issues
- **File system problems**: Mount failures or disk space issues in console/dmesg

**Metrics Format:**
Test artifacts are stored in JSON format with structured data:
```json
{
  "Name": "test-binary-name",
  "Hostname": "test-runner-host",
  "Functions": {
    "TestSSHStartTime": {
      "metrics": [
        {
          "ID": "SSHDStartTime",
          "Value": "24.127942",
          "Data": {
            "arch": "x86_64",
            "cloud": "aws",
            "env": "staging"
          }
        }
      ],
      "files": [
        {
          "ID": "JournalCtlB0",
          "Content": "QXVnIDA1IDE3OjU0OjE1IGNoYWluZ3VhcmQ...",
          "Data": {},
          "Error": null
        }
      ]
    }
  }
}
```

#### Example File and Metric Artifacts

**Available File Artifact IDs** (from `vms-test/pkg/artifacts/files/id_generated.go`):
- `Dmesg`: Kernel boot messages (`dmesg` output)
- `JournalCtlB0`: System journal with boot messages (`journalctl -b 0`)
- `SystemdAnalyze`: Systemd boot analysis (`systemd-analyze` output)
- `SystemdCriticalChain`: Systemd critical chain analysis
- `Proc1Mountinfo`: Process 1 mount information (`/proc/1/mountinfo`)
- `NestedQemuConsole`: Nested QEMU console output

**Available Metric Artifact IDs** (from `vms-test/pkg/artifacts/metrics/id_generated.go`):
- `SSHDStartTime`: Time for SSH daemon to start (seconds)
- `MultiUserTarget`: Time to reach multi-user.target (seconds)
- `SystemdState`: Current systemd system state

#### Test Environment Variables

- `TEST_ARCHES`: Target architectures for testing (x86_64, aarch64)
- `WAIT_SSH_TIMEOUT`: SSH connection timeout (default: 5m)
- `WAIT_CONSOLE_TIMEOUT`: Console output timeout (default: 5m)
- `METD`: Metrics output directory (default: /tmp/chainguard-vmtest-metrics)

## Key Environment Variables

- `ARCH`: Target architecture (x86_64, aarch64)
- `PUBLISH_TARGET`: Publishing environment (dev, staging, eap, production) - defaults to dev
- `PREFIX`: Image name prefix (defaults to username)
- `BUILD_TIMESTAMP`: Timestamp for versioning (auto-generated)
- `COMMIT`: Git commit hash for tagging
- `WVM_SSH_PORT`: SSH port for local VM testing (default: 6379)
- `WVM_DISPLAY`: QEMU display mode for local testing

## File Patterns

- `configs/*/build.yaml`: VM build configurations
- `configs/*/test.yaml`: Test specifications
- `output/*/disk.raw`: Generated raw disk images
- `builder/`: Build dependencies (kernel, initrd, OVMF)
- `tools/*`: Shell scripts for various operations
