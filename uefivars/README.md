# UEFI Variables

Systemd boot supports [UEFI
variables](https://systemd.io/BOOT_LOADER_INTERFACE/) to control
behaviour.

This allows to selecting alternative one-time boot entry, the default
boot entry, menu timeout and so on.

This is very helpful for in-cloud debugging as the same image can be
registered with alternative UEFI variables; to then cause console
access to pre-boot environment.

## Examples

Force boot menu to allow customzing kernel cmdline parameters and selecting alternative boot entries:

On Local machine:

```
$ sudo bootctl set-timeout menu-force
```

However that is only useful if one has already successfully booted. To
set such a variable before booting there is a [python
tool](https://github.com/awslabs/python-uefivars) from AWS that
enables one to serialize/deserialise UEFI variables to/from the
efivars store, json, EDK2 VARS.fd fileformat, and the AWS AMI
registration uefi_data blob.


For example one can dump installed variables with

```
$ uefivars -i efivarfs -o json -I /sys/firmware/efi/efivars
```

This has json for the above menu-force command

```
        {
            "name": "LoaderConfigTimeout",
            "data": "34003200390034003900360037003200390035000000",
            "guid": "4a67b082-0a4c-41cf-b6c7-440b29bb8c4f",
            "attr": 7
        }
```

Which one can convert into EDK2 Vars and into AWS AMI uefi_data.

```
$ uefivars -i json -I LoaderConfigTimeout-menu-force.json -o edk2 -O LoaderConfigTimeout-menu-force.VARS.fd
$ uefivars -i json -I LoaderConfigTimeout-menu-force.json -o aws -O LoaderConfigTimeout-menu-force.aws

This enables to launch the same image, break into boot menu, to for
example set "systemd.debug-shell=ttyS0" instead of "console=ttyS0".
