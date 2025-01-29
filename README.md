# wolfi-vm
The tools in this repo help build Chainguard / Wolfi virtual machine images.

## Getting started
FIXME: First sticking point is that you need to copy a OVMF.fd file into
~/Downloads/.  This is hacky. :cry:.  On ubuntu that is:

    cp /usr/share/ovmf/OVMF.fd ~/Downloads/OVMF-$(uname -m).fd

After that, the easiest thing to see this working is to run:

    make debug-disk-generic

That will:
1. build the image described in 'configs/generic.yaml' into output/configs/disk-debug.raw
2. Boot a qemu guest.  The debug has console output to stdio and a root shell (`systemd.debug-shell`) on ttyS1


## How is the install done?
To do the install this process will:
1. create a `output/<name>/initrd.tar` file that has contents of the image.
   That is done with apko, providing it the file configs/<name>.yaml
2. create a builder initrd from mkvm.yaml
3. start a qemu guest with the downloaded kernel and created builder/initrd.cpio.
   It will have:

    * tools/ mapped in via plan9 (mp=workload)
    * output/<name>/ mapped in via plan9 (mp=output)
    * kernel command line arguments to describe what should be mounted (workload and output)
      and what should be executed (entry=install-target-disk).
    * `output/<name>/initrd.tar` attached as a raw disk (vda)
    * `output/<name>/disk.raw.tmp` attached as raw disk

If you want to make changes to the install, you can hack around in tools/install-target-disk.  See the DEBUG code there that is used by the `debug-disk-*` targets.

## Testing other images
If you want to test building an existing container image type:

    make configs/<name>.json

That will use cosign to fetch a json config for `name` into `configs/<name>-container.json` and create a configs/name.json file that has `BOOT_PKGS` added to the package list.

You can then attempt to build and run that config just as you would one of the existing committed configs.

## Debugging
 * `make run-builder` will execute a builder environment and drop you into a shell.  The /tools should be mounted in at /workload, so you can execute /workload/install-target-disk as you'd like.
 * `make debug-disk-<name>` - boot 'name' with debug output.

 * qemu-vms are booted with `-display none -serial mon:stdio`, which multi-plexes serial console and qemu-monitor on stdout.  Also booted with `-echr 0x5` (which is 'e').  So to switch back and forth between the qemu-monitor and serial console, you can type 'ctrl-e c'.  If you just want to get out of the qemu vm, type `ctrl-e c` and then `quit`
 * You can change the display to qemu with QEMU_DISPLAY, such as QEMU_DISPLAY=sdl or QEMU_DISPLAY=vnc:1
 * Connect to the ttyS1 console in debug witih `socat STDIO,cfmakeraw,isig=1 UNIX:output/NAME/.socket.ttyS1`

 * You can include extra local repos through the `EXTRA_REPO_DIRS` variable.
   'make EXTRA_REPO_DIRS=$HOME/src/wolfi-os' will then consider packages from
   $HOME/src/wolfi-os/packages/<arch>/.

   Note though, that changes to the packages in those directories will not be
   recognized. You will have to remove image.tar to force rebuild.
