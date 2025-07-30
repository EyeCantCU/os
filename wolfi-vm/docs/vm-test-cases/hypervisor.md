# Hypervisor Tests

The `hypervisor` test group validates hypervisor functionality capabilities on VM instances.

## Test Location

- **Source**: `vms-test/pkg/test/hypervisor/`
- **Test Group**: `hypervisor`

## Test Cases

### nestedvirt

#### TestNestedKVM (nestedvirt)
**Purpose**: Verifies that nested virtualization is properly enabled and accessible

**Implementation**:
- Checks that `/dev/kvm` device is created and accessible after module loading
- Validates that the KVM device file exists and can be accessed
