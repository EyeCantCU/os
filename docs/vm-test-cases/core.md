# Core VM Tests

The `core` test group validates fundamental VM functionality required for all deployments.

## Test Location

- **Source**: `vms-test/pkg/test/core/`
- **Test Group**: `core`

## Test Cases

### kernel

#### TestErrorsInDmesg
**Purpose**: Scans kernel messages for critical errors and warnings

**Implementation**:
- Executes `dmesg` to retrieve kernel ring buffer messages
- Searches for kernel oops/panics and warnings

#### TestCollectLogs
**Purpose**: Collects kernel diagnostic information and performance metrics

**Implementation**:
- Waits for `graphical.target` to start
- Records time to reach graphical target as performance metric

### ssh

#### TestSSHStartTime
**Purpose**: Validates SSH service startup and measures performance

**Implementation**:
- Waits for `sshd.service` to reach active state
- Measures and records SSH daemon startup time

### systemd

#### TestSystemdStatus
**Purpose**: Validates systemd init system functionality

**Implementation**:
- Connects to systemd via D-Bus interface
- Waits for systemd to reach "running" state

#### TestCollectLogs
**Purpose**: Collects comprehensive system diagnostic information

**Implementation**:
- Waits for `graphical.target` startup
- Records multi-user target startup time as performance metric
