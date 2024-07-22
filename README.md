# wolfi-vm
The tools in this repo help build Chainguard / Wolfi virtual machine images.

 - ```Makefile``` contains a handful of useful targets to running the helper utilities
 - ```wolfi-vm``` is a top level wrapper script with a number of options and parameters (see: ```wolfi-vm --help```)
 - ```.fetch-linux-kernel``` is a helper tool that will download Linux kernel packages from Chainguard's enterprise-packages repo, and extract the ```vmlinuz``` file
 - ```.mkdiskimg``` is a helper tool that needs to run inside of a privileged Docker container, and will create a bootable disk image
 - ```.mkinitrd``` is a helper tool that builds an initrd filesystem needed by the Linux kernel to boot and run

## Examples:
 - ```wolfi-vm --ephemeral``` will build and launch an ephemeral virtual machine 
 - ```wolfi-vm --disk``` will build and launch a virtual machine backed by a disk image for persistent storage
