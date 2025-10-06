# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is `wolfi-vm`, a project for building virtual machine disk images from apko YAML configurations. It's similar to container building but creates bootable VM images for various cloud platforms (AWS, Azure, GCP) and local QEMU testing.

## Key Components

- **Main CLI**: `apkoaas` binary built from Go code in `main.go` and `pkg/cli/`
- **Build System**: Uses both Makefile and Go-based CLI for VM image creation
- **Image Configs**: YAML files in `configs/` directory define packages and settings for different VM variants
- **Cloud Support**: Platform-specific configurations for AWS, Azure, GCP, and generic QEMU images
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
make disk-generic-base
make disk-aws-base
make disk-azure-docker

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
make run-generic-base  # Boot locally, SSH via port 6379

# Run debug version with console access
make run-debug-<config-name>
make debug-shell-<config-name>  # Connect to debug console

# Run comprehensive tests
make test-gotest            # Go unit tests
make test-generic           # Full VM testing on generic image
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

Publishing is controlled by the `PUBLISH_TARGET` environment variable which determines the target cloud environment and resources.

#### Publishing Environments

The system supports four publishing environments:

1. **dev** (default): Development environment using your personal GCP project
2. **staging**: Staging environment for pre-production testing
3. **eap**: Early Access Program (production-level for EAP users)
4. **production**: Production environment

#### Publishing Commands

```bash
# Publish to development (default)
make publish-azure-<image-name>
make publish-gcp-<image-name>
make publish-qemu-<image-name>

# Publish to staging
PUBLISH_TARGET=staging make publish-azure-<image-name>
PUBLISH_TARGET=staging make publish-gcp-<image-name>

# Publish to EAP
PUBLISH_TARGET=eap make publish-azure-<image-name>
PUBLISH_TARGET=eap make publish-gcp-<image-name>

# AWS publishing (uses awspub configuration)
make aws-create-<image-name>
make aws-publish-<image-name>

# Examples:
PUBLISH_TARGET=dev make publish-gcp-base
PUBLISH_TARGET=staging make publish-azure-docker
PUBLISH_TARGET=eap make publish-qemu-base
```

#### Publishing Output Files

Published images create tracking files:
- Azure: `output/<arch>/<image>/publish.<env>.json`
- GCP: `output/<arch>/<image>/publish.<env>.yaml`
- QEMU: `output/<arch>/<image>/publish.<env>.json`
- AWS: `output/<arch>/awspub/publish/<image>.output`

#### Additional Publishing Variables

- `PREFIX`: Image name prefix (defaults to username via `id -un`)
- `BUILD_TIMESTAMP`: Timestamp for image versioning (auto-generated)
- `COMMIT`: Git commit hash for tagging
- `AZVERSION`: Azure-specific version format

## Configuration Structure

Each config in `configs/` contains:
- `build.yaml`: Apko configuration defining packages, repositories, architecture
- `test.yaml`: Test specifications for validation

Example config structure:
```
configs/aws-base/
├── build.yaml    # Package list, repos, arch settings
└── test.yaml     # Test cases for this image variant
```

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
# Run comprehensive VM testing on generic image
make test-generic

# Build and run tests for specific architecture
make -C vms-test TEST_ARCHES="x86_64" runners tests

# Run tests using helpers (manual execution)
QEMU_VMS="generic-base" ./vms-test/helpers/test-wolfi-vm --wolfi-vm $PWD qemu ./test-results
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
- Example input: `output/x86_64/awspub/create/aws-base.json`

**Azure Image Selection:**
- Uses published image information from Azure galleries
- Supports latest versions or specific image versions
- Configured via gallery name, resource group, and image name parameters

**GCP Image Selection:**
- Uses `publish.<env>.yaml` files from `output/<arch>/gcp-*/publish.<env>.yaml`
- Extracts image URIs and metadata from YAML output
- Supports filtering by project and image family

**QEMU Testing:**
- Uses local disk images from `output/<arch>/generic-*/disk.raw`
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
   # Example: if aws-base tests fail, examine configs/aws-base/build.yaml
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
