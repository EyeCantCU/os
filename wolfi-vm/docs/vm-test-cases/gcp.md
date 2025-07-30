# GCP Platform Tests

The `gcp` test group validates Google Cloud Platform-specific platform integration and functionality.

## Test Location

- **Source**: `vms-test/pkg/test/gcp/`
- **Test Group**: `gcp`

## Test Cases

### routing

#### TestRoutePresence
**Purpose**: Validates network routing

**Implementation**:
- Uses netlink to query routing table for Google MDS IP
- Tests routes to `169.254.169.254` (Google Metadata Service)
- Confirms at least one route exists to the metadata endpoint

### startupscripts

#### TestStartupScripts
**Purpose**: Validates Google Cloud startup script execution

**Implementation**:
- Expects VM to be started with startup script metadata that creates test file
- Reads `/tmp/hello-wolfi.txt` file created by startup script
- Verifies file contains expected "hello wolfi" content
