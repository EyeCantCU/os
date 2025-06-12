# Test Configuration V1

This file specifies gcp-specific options in test configuration v1. Refer to configuration.md for generic options.

* **cloud**: must be gcp.
* **data**:
	* **zone**: *Suggested* zone. The user may choose to change this at runtime, do not rely on it.
* **vmconfigs**:
	* **x86_64**:
	* **aarch64**:
		* **name** (optional): friendly name for the vm configuration, default is machine_type-disk_type
		* **machine_type**: Machine type. See https://cloud.google.com/compute/docs/machine-resource
		* **disk_type**: Disk type. See https://cloud.google.com/compute/docs/disks
		* **disk_size**: Boot disk size in GB, default 10
		* **min_cpu_platform** (optional): Minimum CPU platform for instance
		* **nested_virt** (optional): Enable nested virtualization for instance
		* **metadata**: String containing extra metadata, in 'key=value;' format

## Non-normative conventions

This section documents conventions that are true at time of writing and helpful for authoring tests and configurations, but are subject to change without bumping configuration version.

### Finding test configuration files

Finding test configuration files is done by checking the local-name label on an image, and running the test configuration at `configs/$local-name/test.yaml` in wolfi-vm.

## test.yaml example

```yaml
version: 1
cloud: gcp
data:
  vmuser: root
  zone: us-central1-a

vmconfigs:
  x86_64:
    - machine_type: e2-medium
      disk_type: pd-balanced
      metadata: 'enable-oslogin=true'
      tests:
        - group: core
    - machine_type: c4-standard-4
      disk_type: hyperdisk-balanced
      tests:
        - group: core
  aarch64:
    - machine_type: t2a-standard-4
      disk_type: pd-balanced
      tests:
        - group: core
    - machine_type: c4a-standard-4
      disk_type: hyperdisk-balanced
      tests:
        - group: core
```
