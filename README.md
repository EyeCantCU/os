# wolfi-vm

The tools in this repo help build Chainguard / Wolfi virtual machine images.

  - ```Makefile``` contains a handful of useful targets to running the helper utilities
  - ```wolfi-vm``` is a top level wrapper script with a number of options and parameters (see: ```wolfi-vm --help```)
  - ```.mkdiskimg``` is a helper tool that needs to run inside of a privileged Docker container, and will create a bootable disk image
  - ```.mkinitrd``` is a helper tool that builds an initrd filesystem needed by the Linux kernel to boot and run

## Examples:

  - ```SSH_KEYS=/path/to/id_rsa.pub DOCKER_IMAGE=generic make image```  will create a generic system with apk and systemd
  - ```SSH_KEYS=/path/to/id_rsa.pub DOCKER_IMAGE=docker-runner make image```  will create a generic system with apk, systemd and working docker/containerd
  - ```SSH_KEYS=/path/to/id_rsa.pub DOCKER_IMAGE=foo make initrd``` will create an initrd of a chainguard docker image
  - ```SSH_KEYS=/path/to/id_rsa.pub DOCKER_IMAGE=foo make image``` will create an system of a chainguard docker image
  - ```DOCKER_IMAGE=foo make google-image-upload``` will upload generated image raw.tar.gz to gcp
  - ```DOCKER_IMAGE=foo make azure-image-upload``` will upload generated image vhd to azure
  - ```DOCKER_IMAGE=foo make aws-image-upload``` will upload generated image vhd to aws

## Working examples:

### Create a VM with docker and upload it to GCP

```console
DOCKER_IMAGE=docker-runner SSH_KEYS=/path/to/id_rsa.pub make image
DOCKER_IMAGE=docker-runner google-image-upload
```

### Run the image locally with QEMU

```console
./wolfi-vm -d ./disk.docker-runner.raw
```

#### Notes

VHD images can be used also with Virtual Box, you can use the Azure image to
test with Virtual Box locally
