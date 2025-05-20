# Test Configuration V1

This file specifies azure-specific options in test configuration v1. Refer to configuration.md for generic options.

* **cloud**: must be azure.
* **data**:
	* **vmuser**: If unspecified, this will default to "azureuser".
	* **region**: *Suggested* region. The user may choose to change this at runtime, do not rely on it.
	* **launchuser**: The username to set as the admin username during VM launch, if different from the user to run tests as. If unspecified, user will be used.
* **vmconfigs**:
	* **x86_64**:
	* **aarch64**:
		* **name** (optional): friendly name for the vm configuration, default is vm_size-disk_type
		* **vm_size**: VM size. See https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/overview
		* **disk_type**: Disk type. See https://learn.microsoft.com/en-us/azure/virtual-machines/disks-types
		* **tests**: This key contains a list of test configurations to be run on the VM.
			* **launchuser**: The username to set as the admin username during VM launch, if different from the user to run tests as. If unspecified, inheritance order is top level launchuser -> test user -> top level user.

## Non-normative conventions

This section documents conventions that are true at time of writing and helpful for authoring tests and configurations, but are subject to change without bumping configuration version.

### Finding test configuration files

Finding test configuration files is done by checking the local-name tag on an image, and running the test configuration at `configs/$local-name/test.yaml` in wolfi-vm.

## test.yaml example

```yaml
version: 1
cloud: azure
data:
  launchuser: azureuser
  vmuser: root
  region: eastus

vmconfigs:
  x86_64:
    - vm_size: Standard_F2s_v2
      disk_type: StandardSSD_LRS
      tests:
        - group: core
    - vm_size: Standard_F2as_v6
      disk_type: StandardSSD_LRS
      tests:
        - group: core
  aarch64:
    - vm_size: Standard_D2ps_v6
      disk_type: StandardSSD_LRS
      tests:
        - group: core
    - vm_size: Standard_E2ps_v6
      disk_type: StandardSSD_LRS
      tests:
        - group: core
```
