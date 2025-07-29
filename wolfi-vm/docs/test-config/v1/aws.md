# Test Configuration V1

This file specifies aws-specific options in test configuration v1. Refer to configuration.md for generic options.

* **cloud**: must be aws.
* **data**:
	* **vmuser**: If unspecified, this will default to "ec2-user".
* **vmconfigs**:
	* **x86_64**:
	* **aarch64**:
		* **name** (optional): friendly name for the vm configuration, default is instance_type
		* **instance_type**: EC2 instance type. See https://aws.amazon.com/ec2/instance-types/

## Non-normative conventions

This section documents conventions that are true at time of writing and helpful for authoring tests and configurations, but are subject to change without bumping configuration version.

### Finding test configuration files

Finding test configuration files is done by assuming that an awspub output named `$arch/awspub/$name.json` has a corresponding config file `configs/$name/test.yaml`.

## test.yaml example

```yaml
version: 1
cloud: aws
data:
  vmuser: aws

vmconfigs:
  x86_64:
    - instance_type: t3.medium
      tests:
        - group: core
  aarch64:
    - instance_type: t4g.medium
      tests:
        - group: core
```
