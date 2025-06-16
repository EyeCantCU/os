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
        * **secondary_disk_name** (optional): Optional device name for the secondary disk, defaults to instance_name-data
        * **secondary_disk_size** (optional): Size of secondary data disk in GB, default 0 (no secondary disk)
        * **secondary_disk_type** (optional): Disk type for secondary disk, defaults to same as disk_type
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
      secondary_disk_size: 50
      secondary_disk_type: pd-ssd
      metadata: 'enable-oslogin=true'
      tests:
        - group: core
    - machine_type: c4-standard-2
      disk_type: hyperdisk-balanced
      tests:
        - group: core
        - group: gcp
  aarch64:
    - machine_type: t2a-standard-2
      disk_type: pd-balanced
      tests:
        - group: core
    - machine_type: c4a-standard-2
      disk_type: hyperdisk-balanced
      secondary_disk_name: database1
      secondary_disk_size: 100
      tests:
        - group: core
```
