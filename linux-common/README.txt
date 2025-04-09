
This README describes configuration maintenance.

There are a number of default configurations that are
applied to all cloud instance types that are architecture
and cloud independent (pipelines/kernel/config.yaml).

Each cloud instance type also have some settings that are
unique to the cloud. For example, each cloud has their own
virtual networking device, i.e., CONFIG_ENA, CONFIG_GCE. Those
configurations are applied in the cloud instance YAML file.
There are also hypervisor settings that are specific to a
cloud, i.e., KVM, HYPERV, and XEN.

The primary configuration file for each cloud type is best
maintained using 'make menuconfig ARCH=$ARCH" where arch is
x86 or arm64. For example, this is how you would update
the Google arm64 configuration:

cd linux-stable
cp ~/Chainguard/enterprise-packages/linux-common/config-gcp-generic-arm .config
make menuconfig ARCH=arm64 # Make your changes here. Select "Save" when exiting
cp .config ~/Chainguard/enterprise-packages/linux-common/config-gcp-generic-arm
 
There are also scripts in the kernel repository that you can use to interrogate and manipulate
the kernel config:

scripts/config
scripts/diffconfig

Tim Gardner
March 24, 2025
