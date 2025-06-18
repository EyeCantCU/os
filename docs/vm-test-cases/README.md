# VM Test Cases Documentation

This directory contains documentation for VM test cases. These docs help engineers understand what is being tested in each test case, basic implementation details, and the rationale for test group organization.

## Documentation Purpose

Each test group documentation provides:
- **Test overview**: Purpose and scope of the test group
- **Test location**: Source code
- **Test cases**: What each test validates and basic implementation approach
- **Group rationale**: Why these tests are organized into their respective groups

## Generating New Documentation

When creating documentation for new test groups, use this Claude prompt or add this to your CLAUDE.md:

```
Create documentation for the [TEST_GROUP_NAME] test group following the wolfi-vm test documentation format.

Requirements:
- Help engineers understand what is being tested, basic implementation details, and why tests are grouped together
- Follow this exact structure:

# [Test Group Name] Tests

[One sentence describing what this test group validates and why it exists as a group]

## Test Location

- **Source**: `vms-test/pkg/test/[path]/`
- **Test Group**: `[group_name]` (or **Test Name**: `[name]` for individual tests)

## Test Cases

### [test_package_name]

#### [TestFunctionName]
**Purpose**: [What this test validates in one sentence]

**Implementation**:
- [Key step 1]
- [Key step 2]
- [Key step 3]

Be concise - focus on WHAT is tested and basic HOW, not detailed troubleshooting or examples.

Based on the source code in `vms-test/pkg/test/[path]/`, generate this documentation.
```

## Configuration Reference

Test groups are configured in `configs/test/{platform}/{variant}.yaml` files:

```yaml
vmconfigs:
  x86_64:
    - machine_type: standard-type
      tests:
        - group: core        # Always include for basic validation
        - group: containers  # For Docker-enabled images
        - group: azure      # For Azure platform (Azure only)
        - group: hypervisor  # For VMs with nested_virt: true
        - name: applications/qemu  # Individual QEMU application test
```
