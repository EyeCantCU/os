# Test Configuration V1

Each image built or published should have a configuration file somewhere which describes how the image should be tested to determine whether it is suitable for publishing to prod.

Test helpers in this repository will read this configuration and run tests according to the options within in it.
Logic for deciding what image maps to a configuration file is not part of the test configuration details and not covered by this.

The following options are part of this version of the test configuration specification.

* **version**: The version of configuration options. This specification is for version 1.
* **cloud**: the cloud this image is intended for (one of qemu, azure, aws, or gcp).
* **data**: top level configuration details.
	* **vmuser**: The user to connect to the VM with. Tests are run with root permissions.
	* Any other fields under this key are considered a cloud specific definition.
* **vmconfigs**: Contains a list of architectures for which VM configs and tests are defined.
	* **x86_64**: This field is a list of vm configurations on x86_64.
	* **aarch64**: This field is a list of vm configurations on aarch64.
		* **launch** (optional): If specified, this binary will be called to launch the VM under test rather than the “runner/$cloud launch” command. Arguments provided to the binary will be the same arguments that would have been provided to the runner. The 'RUNNER' environment variable points to the binary that would have been run.
		* **user_data** (optional): If specified, the file content at the given path will be passed as "user data"/"custom data"/"meta data" during instance launch. Accepts a path relative to the test config.
		* **tests**: This key contains a list of test configurations to be run on the VM.
			* **group** (exclusive with name): This is the name of a test group, all members of which should be run.
			* **name** (exclusive with group): This is the name of an individual test which should be run. Specifying individual tests in a test group as $group/$name is permitted. If this test is not part of any test group, location is required.
			* **vmuser** (optional): This the name of the user to connect to the VM as, if different from top level definition.
			* **location** (optional): This is the location on the filesystem (relative to the file containing the test configuration) of a folder containing a go package defining a test. It will be compiled in the same manner as built-in tests are compiled. The compiler tag 'vmtest' can be used to detect when being compiled for execution on the target.
		* All other fields under this key are considered a cloud specific definition.

## Non-normative conventions

This section documents conventions that are true at time of writing and helpful for authoring tests and configurations, but are subject to change without bumping configuration version.

### Finding test configuration files

Currently, finding test configuration files is done by trying to determine the name of the build configuration that built the image (this logic is cloud specific), and running the test configuration at `configs/$name/test.yaml` in wolfi-vm.

## test.yaml example

```yaml
version: 1
cloud: fixme # one of qemu, azure, aws, gcp
data:
  vmuser: root

vmconfigs:
  x86_64:
    - cloud_specific_config_option: x86_64-data
      tests:
        - group: core
  aarch64:
    - cloud_specific_config_option: aarch64-data
      tests:
        - name: custom-test
          location: ./custom-test
```
