# Test Configuration V1

This file specifies qemu-specific options in test configuration v1. Refer to configuration.md for generic options.

* **cloud**: must be qemu.
* **data**:
	* **vmuser**: If unspecified, this will default to "linky".
* **vmconfigs**:
	* **x86_64**:
	* **aarch64**:
		* **name** (optional): friendly name for the vm configuration, default is fwcode basename
		* **fwcode**: path (relative to test.yaml) to fwcode

## Non-normative conventions

This section documents conventions that are true at time of writing and helpful for authoring tests and configurations, but are subject to change without bumping configuration version.

### Finding test configuration files

Finding test configuration files is done by finding vmnames in output/$testarch/*, and assuming the config lives at `configs/$vmname/test.yaml`.

## test.yaml example

```yaml
version: 1
cloud: qemu
data:
  vmuser: linky

vmconfigs:
  x86_64:
    - fwcode: ../../builder/ovmf-x86_64.fd
      tests:
      - group: core
  aarch64:
    - fwcode: ../../builder/ovmf-aarch64.fd
      tests:
      - group: core
```
