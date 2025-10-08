# Build your own VM
Chainguard Virtual Machines are build in a very similar fashion to how containers are built. Apko creates a tarball that is then converted into a raw disk image.

## Create your VM Config file
A VM config file is just a apko YAML file that describes the packages that are part of your virtual machine images.

These files are stored under `configs` folder.

## Build your VM
Once you have created a config, you can run the following command:

```
make disk-<your config>
```

For example, the following command will build a raw disk image from the `configs/generic.yaml` file.

```
make disk-generic
```

## Build RPi images

Build using:

```
make ARCH=aarch64 disk-rpi-generic-base
make ARCH=aarch64 disk-rpi-generic-docker
```

Flash them to sdcard using:

```
dd if=output/aarch64/rpi-generic-base/disk.raw of=/dev/sdX conv=fsync status=progress
dd if=output/aarch64/rpi-generic-docker/disk.raw of=/dev/sdX conv=fsync status=progress
```

Or the official RPi imager that can be found [here](https://www.raspberrypi.com/software/)

## Running your VM
Images built for a cloud have a kernel and packages built for that platform.  They aren't necessarily of any use inside qemu.

That being said, to test a VM locally, you can make similar changes to a qemu image (like 'generic.yaml') and boot it.

You can test run your VM with:

    make run-<your config>

As an example:

    make run-generic

That will boot a VM with the generic disk image.  You can watch it boot and will get a login prompt on your terminal.

There are no builtin passwords, so you won't be able to log in. :cry:

2 things make it possible to get in.

1. [qemu-guesthelper](https://github.com/chainguard-dev/enterprise-packages/blob/main/qemu-guesthelper/README.md)

   The `make run-<vmname>` will supply ssh keys to the guest vm via smbios in a way that
   A vm that has `qemu-guesthelper-authorized-keys-command` installed can read.

   So if your vm has that package installed (like 'generic') then you can do:

       make run-generic

   And then switch to another terminal and

       ssh -p6379 linky@localhost

   The linky user will have passwordless sudo to be root.

2. `make run-debug-<name>`

   This will create a 'debug-<name>' image that has systemd-debug.shell enabled.
   The terminal you run that in will boot and show a login prompt, but you can
   then switch to another terminal and type: `make debug-shell-<name>` in order
   to be placed into the vm in a root shell.


## Modifying run targets
You can influence the qemu invocation created by `make run-<vmname>` with the following environment variables:

 * `WVM_SSH_PORT`: forward localhost:WVM_SSH_PORT to guest's port 22.  Default is 6379.
   When ssh'ing to vms on localhost, it may be useful to configure `NoHostAuthenticationForLocalhost yes` in your ssh config.

 * `WVM_DISPLAY`: use `-display` instead of default `none`.  See qemu doc for other values. A useful value might be `sdl` or `vnc`

 * `WVM_VNC`: Start qemu with `-vnc` value other than the default `none`.  For example to listen on localhost port 5900 (vnc `:0`) you can set `WVM_VNC=localhost:0` and then connect to that with your vnc client.

## Adding a "backdoor" to an image.
It can be tricky to figure out what is going wrong in a VM if you can't get log into it.

The `./tools/backdoor-image` script will insert a user named `backdoor` into the image
and can add some public keys to the user's .ssh/authorized_keys.  The user will also
have sudo access.

To add your .ssh/id_ed25519.pub key into the vm:

    sudo ./tools/backdoor-image --pubkeys ~/.ssh/id_ed25519.pub output/x86_64/generic/disk.raw

To insert github user 'smoser' public keys:

    sudo ./tools/backdoor-image --import-id=smoser output/x86_64/aws-base/disk.raw

Just run the script on your image before publishing (or before running `make run-<vmname>`)
and you should then be able to ssh in as the 'backdoor' user.

## Test
Run `make test` to test builder logic and run other tests.

## Converting existing OCI images

[convert.txt](./convert.txt) contains an allowlist of files
which will convert some of our images into VMs.

This is done by downloading the apko attestation and injecting a few extra apks, then building with `apko build-minirootfs`

Then, to build them, you can run
```
make convert
```

Then checkout the `converted/` directory.
