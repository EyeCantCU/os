# Build your own VM
Chainguard Virtual Machines are build in a very similar fashion to how containers are built. Apko creates a tarball that is then converted into a raw disk image.

## Create your VM Config file
A VM config file is just a apko YAML file that describes the packages that are part of your virtual machine images.

These files are stored under `configs` folder.

## Build your VM
Once you have created a config, you can run the following command:

```
make disk-<your config>
```

For example, the following command will build a raw disk image from the `configs/qemu-base-slim/build.yaml` file.

```
make disk-qemu-base-slim
```

## Build RPi images

Build using:

```
make ARCH=aarch64 disk-rpi-base-slim
make ARCH=aarch64 disk-rpi-docker-slim
```

Flash them to sdcard using:

```
dd if=output/aarch64/rpi-base-slim/disk.raw of=/dev/sdX conv=fsync status=progress
dd if=output/aarch64/rpi-docker-slim/disk.raw of=/dev/sdX conv=fsync status=progress
```

Or the official RPi imager that can be found [here](https://www.raspberrypi.com/software/)

## Add local package directories
To build an image with with local stereo package directories, modify the desired `configs/<name>/build.yaml` file by
1. Append the following entries to the existing `config.build_respositories` key:

   ```yaml
   build_repositories:
     - ../enterprise-packages/packages
     - ../extra-packages/packages
     - ../os/packages
   ```
2. Add the following `config.keyring` key, or append if it exists:

   ```yaml
   keyring:
     - ../enterprise-packages/local-melange-enterprise.rsa.pub
     - ../extra-packages/local-melange-extra.rsa.pub
     - ../os/local-melange.rsa.pub
   ```

## Running your VM
Images built for a cloud have a kernel and packages built for that platform.  They aren't necessarily of any use inside qemu.

That being said, to test a VM locally, you can make similar changes to a qemu image (like 'qemu-base-slim/build.yaml') and boot it.

You can test run your VM with:

    make run-<your config>

As an example:

    make run-qemu-base-slim

That will boot a VM with the qemu-base-slim disk image.  You can watch it boot and will get a login prompt on your terminal.

There are no builtin passwords, so you won't be able to log in. :cry:

2 things make it possible to get in.

1. [qemu-guesthelper](https://github.com/chainguard-dev/enterprise-packages/blob/main/qemu-guesthelper/README.md)

   The `make run-<vmname>` will supply ssh keys to the guest vm via smbios in a way that
   A vm that has `qemu-guesthelper-authorized-keys-command` installed can read.

   So if your vm has that package installed (like 'qemu-base-slim' does) then you can do:

       make run-qemu-base-slim

   And then switch to another terminal and

       ssh -p6379 linky@localhost

   The linky user will have passwordless sudo to be root.

2. `make run-debug-<name>`

   This will create a 'debug-<name>' image that has systemd-debug.shell enabled.
   The terminal you run that in will boot and show a login prompt, but you can
   then switch to another terminal and type: `make debug-shell-<name>` in order
   to be placed into the vm in a root shell.


## Modifying run targets
You can influence the qemu invocation created by `make run-<vmname>` with the following environment variables:

 * `WVM_SSH_PORT`: forward localhost:WVM_SSH_PORT to guest's port 22.  Default is 6379.
   When ssh'ing to vms on localhost, it may be useful to configure `NoHostAuthenticationForLocalhost yes` in your ssh config.

 * `WVM_DISPLAY`: use `-display` instead of default `none`.  See qemu doc for other values. A useful value might be `sdl` or `vnc`

 * `WVM_VNC`: Start qemu with `-vnc` value other than the default `none`.  For example to listen on localhost port 5900 (vnc `:0`) you can set `WVM_VNC=localhost:0` and then connect to that with your vnc client.

 * `WVM_TPM`: Start qemu with tpm emulation, using swtpm. Value of 1 will enable it, not specifying or value 0 will not enable it.for example `WVM_TPM=1`

## Adding a "backdoor" to an image.
It can be tricky to figure out what is going wrong in a VM if you can't get log into it.

The `./tools/backdoor-image` script will insert a user named `backdoor` into the image
and can add some public keys to the user's .ssh/authorized_keys.  The user will also
have sudo access.

To add your .ssh/id_ed25519.pub key into the vm:

    sudo ./tools/backdoor-image --pubkeys ~/.ssh/id_ed25519.pub output/x86_64/qemu-base-slim/disk.raw

To insert github user 'smoser' public keys:

    sudo ./tools/backdoor-image --import-id=smoser output/x86_64/aws-base-slim/disk.raw

Just run the script on your image before publishing (or before running `make run-<vmname>`)
and you should then be able to ssh in as the 'backdoor' user.

## Publishing to OCI Registry

VM artifacts (disk images in multiple formats, apko tarballs, SBOMs, and UEFI files) can be published to an OCI registry like cgr.dev or any Docker-compatible registry.

### Config-Driven Publishing

Publishing requires a `publish.yaml` config file that defines the OCI image name, tags, and disk formats.

**Create a publish.yaml config:**

```yaml
version: 1
cloud: azure
name: azure-python-313-slim
oci_config:
  image: python                   # Image name (combined with --registry)
  tags:
    - azure-python-3.13-slim      # Base tag name
    - azure-python-slim           # Each tag becomes 2 variants (see below)
  disk_formats:                   # Required: specify disk formats to publish
    - vhd
    - vmdk
```

**Tag Expansion:**

Each tag in the config is automatically expanded into 2 variants:
- **Timestamped**: `tag-TIMESTAMP` (e.g., `azure-python-slim-20241103-1234`)
- **Latest**: `tag-latest` (e.g., `azure-python-slim-latest`)

The timestamp is auto-generated (format: `YYYYMMDD-HHMM`) or can be specified with `--timestamp`.

Example: The tags above create 4 OCI tags:
- `azure-python-3.13-slim-20241103-1234`
- `azure-python-3.13-slim-latest`
- `azure-python-slim-20241103-1234`
- `azure-python-slim-latest`

**Controlling Disk Formats:**

The `disk_formats` field is required and explicitly controls which disk formats are published:
- Specify exactly which formats you want in the published artifacts (e.g., `[vhd]`, `[qcow2]`, `[raw, vmdk]`)
- All formats specified must exist in the output directory
- Common format combinations:
  - AWS: `[vmdk]` - optimal for EC2 AMI import
  - Azure: `[vhd]` - required for Azure VM images
  - GCP: `[raw]` - used by GCP Compute Engine
  - QEMU: `[qcow2]` - efficient for local testing
  - VMware: `[ova]` - VMware formats
  - Multi-cloud: `[raw, qcow2, vmdk, vhd]` - publish multiple formats

**Platform Naming Requirements:**

Configuration directories must follow the `{platform}-{application}` naming pattern:

**Valid platform prefixes:**
- `aws` - Amazon Web Services
- `azure` - Microsoft Azure
- `gcp` - Google Cloud Platform
- `vmware` - VMware
- `qemu` - QEMU images
- `rpi` - Raspberry Pi

**Examples:**
- ✅ Valid: `aws-base-full`, `azure-docker-slim`, `qemu-base-full`, `gcp-nginx-slim`
- ❌ Invalid: `unknown-base`, `test`, `foo-bar`, `myapp` (no platform prefix)

### How Publishing Works

Each architecture is published separately and automatically merged into the existing multi-arch
index. This allows parallel CI/CD builds where different runners can publish different
architectures concurrently.

**Key Features:**
- Each architecture is published independently
- New architectures automatically merge into existing multi-arch OCI index
- Safe for CI/CD pipelines with parallel arch-specific runners
- Creates new index if none exists
- **Race condition protection**: Automatically detects and merges concurrent publishes

#### Concurrent Publish Handling

The system includes automatic race condition handling for parallel publishes:

1. **Detection**: Before applying tags, the system re-checks the registry for concurrent publishes
2. **Merging**: If another process published while we were building, indexes are automatically merged
3. **Preservation**: Final multi-arch index contains all architectures from both publishes
4. **CI/CD Safe**: Parallel runners building different architectures will result in a properly merged multi-arch index

**Example scenario:**
- CI Runner 1 (x86_64) and Runner 2 (aarch64) publish simultaneously
- Runner 1 publishes x86_64 index → Runner 2 publishes aarch64 index
- Before applying tags, Runner 2 detects Runner 1's index
- Runner 2 merges both indexes and publishes combined result
- Final index contains both x86_64 and aarch64 architectures

### Publishing with Makefile

```bash
# Build and publish current architecture (default: x86_64)
make vhd-azure-python-313-slim
make publish-registry-azure-python-313-slim

# Other examples
make publish-registry-qemu-base-slim
make publish-registry-aws-base-slim

# Publish different architecture
ARCH=aarch64 make publish-registry-azure-docker-full

# Publish with custom registry and timestamp
REGISTRY_REPO=cgr.dev/custom-org BUILD_TIMESTAMP=20250115-1200 make publish-registry-gcp-nginx-full

# Parallel CI/CD workflow example:
# Runner 1: ARCH=x86_64 make disk-aws-base && make publish-registry-aws-base
# Runner 2: ARCH=aarch64 make disk-aws-base && make publish-registry-aws-base
# Result: Multi-arch index with both x86_64 and aarch64
```

### Publishing with CLI

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

**Required flags:**
- `--config` - Path to publish.yaml config file
- `--output-dir` - Directory containing built artifacts
- `--registry` - Registry prefix (e.g., `cgr.dev/chainguard-vms`)

**Optional flags:**
- `--architecture` - Architecture to publish (default: `x86_64`)
- `--timestamp` - Timestamp for tag expansion (default: auto-generated as `YYYYMMDD-HHMM`)
- `--sign-and-attest` - Sign images with cosign


### OCI Image Structure

Published artifacts use a **nested OCI Image Index structure**:

```
Top-Level OCI Image Index (tagged :latest)
├── Platform: amd64/linux → Sub-Index
│   └── Sub-Index for x86_64
│       ├── Image: apko rootfs (artifact.type=apko.v1)
│       ├── Image: disk.raw (artifact.type=disk.raw.v1)
│       ├── Image: disk.qcow2 (artifact.type=disk.qcow2.v1)
│       ├── Image: disk.vmdk (artifact.type=disk.vmdk.v1)
│       ├── Image: disk.vhd (artifact.type=disk.vhd.v1)
│       ├── Image: SBOM (artifact.type=apko-sbom.v1, syft-sbom.v1)
│       └── Image: Secure Boot files (artifact.type=secureboot.v1)
│
└── Platform: arm64/linux → Sub-Index
    └── (same structure for aarch64)
```

**Features**:
- **Standard Platform Selection**: Pull the image and OCI tooling automatically selects your platform
- **Separate Images per Artifact**: Each disk format, SBOM, and file collection is a distinct image
- **Artifact Type Annotations**: Images are tagged with `org.chainguard.vm.artifact.type` for identification
- **Nested Indexes**: OCI-compliant nested structure groups artifacts by architecture first
- **OCI Artifact Compliance**: Non-container artifacts (disk formats, SBOMs, secure boot files) use empty config (`{}`) with media type `application/vnd.oci.empty.v1+json` per [OCI spec guidelines](https://github.com/opencontainers/image-spec/blob/main/manifest.md#guidelines-for-artifact-usage). Only the apko rootfs maintains a full container image config.

**Artifact Type Images**:
- `apko.v1` - Apko rootfs tarball (`application/vnd.oci.image.layer.v1.tar+gzip`)
- `disk.raw.v1` - Raw disk image in tarball (`application/vnd.chainguard.vm.disk.raw.v1+gzip`)
- `disk.qcow2.v1` - QEMU qcow2 format (`application/vnd.chainguard.vm.disk.qcow2.v1+gzip`)
- `disk.vmdk.v1` - VMware VMDK format (`application/vnd.chainguard.vm.disk.vmdk.v1+gzip`)
- `disk.vhd.v1` - Azure VHD/VPC format (`application/vnd.chainguard.vm.disk.vhd.v1+gzip`)
- `disk.ova.v1` - VMware OVA format (`application/vnd.chainguard.vm.disk.ova.v1+gzip`)
- `apko-sbom.v1` - Apko SBOM (`application/spdx+json`)
- `syft-sbom.v1` - Syft SBOM (`application/vnd.chainguard.vm.syft.sbom.json.v1+gzip`)
- `secureboot.v1` - UEFI/secure boot files (`application/vnd.chainguard.vm.secureboot.v1+gzip`)

**Common Annotations**:
- `org.opencontainers.image.created` - Build timestamp
- `org.opencontainers.image.architecture` - Target architecture (x86_64, aarch64)
- `org.chainguard.vm.artifact.type` - Artifact type identifier

**Note**: Unknown files in the output directory are logged as warnings but NOT published. Only explicitly recognized artifact types are included in the published index.

## Test
Run `make test` to test builder logic and run other tests.

There are more vm tests in vms-test/ directory.  To run those tests for a given image, you
can run `make test-<image-name>`.  As example, try running

    make test-qemu-base-slim

That will create test-results/ with the test output.

## Converting existing OCI images

[convert.txt](./convert.txt) contains an allowlist of files
which will convert some of our images into VMs.

This is done by downloading the apko attestation and injecting a few extra apks, then building with `apko build-minirootfs`

Then, to build them, you can run
```
make convert
```

Then checkout the `converted/` directory.
