# Azure Platform Tests

The `azure` test group validates Azure-specific platform integration and functionality.

## Test Location

- **Source**: `vms-test/pkg/test/azure/`
- **Test Group**: `azure`

## Test Cases

### resourcedisk

#### TestResourceDisk
**Purpose**: Validates Azure temporary/ephemeral disk functionality and WALinuxAgent integration

**Implementation**:
- Resolves `/dev/disk/azure/resource` symlink to actual device
- Identifies where the resource disk is mounted
- Confirms presence of `DATALOSS_WARNING_README.txt` warning file

### routing

#### TestAzurePlatformRoutes
**Purpose**: Validates network routing to critical Azure platform services

**Implementation**:
- Uses netlink to query routing table for platform IPs
- Tests routes to `168.63.129.16` (Azure platform services)
- Tests routes to `169.254.169.254` (Azure Instance Metadata Service)
