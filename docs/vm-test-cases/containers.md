# Container Tests

The `containers` test group validates container runtime functionality and Docker operations.

## Test Location

- **Source**: `vms-test/pkg/test/containers/docker/`
- **Test Group**: `containers`

## Test Cases

### docker

#### TestDockerPullAndRun
**Purpose**: Validates basic Docker functionality with Wolfi container images

**Implementation**:
- Downloads and executes `cgr.dev/chainguard/wolfi-base:latest`
- Runs `echo "Hello Wolfi"` inside the container
- Validates exact output match
