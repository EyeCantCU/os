# QEMU Application Tests

The QEMU application tests validate QEMU emulation and nested VM capabilities.

## Test Location

- **Source**: `vms-test/pkg/test/applications/qemu/`
- **Test Name**: `applications/qemu`

## Test Cases

### qemu

#### TestQemuX86
**Purpose**: Tests native x86_64 QEMU virtualization with KVM acceleration

**Implementation**:
- Creates minimal blank disk image (1MB)
- Locates OVMF firmware for x86_64
- Launches QEMU VM with KVM acceleration

#### TestQemuAarch64
**Purpose**: Tests ARM64 virtualization with native KVM or cross-architecture emulation

**Implementation**:
- Creates minimal blank disk image (1MB)
- Locates OVMF firmware for aarch64
- Launches QEMU with appropriate acceleration (KVM or TCG)
