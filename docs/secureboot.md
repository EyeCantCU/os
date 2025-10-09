# Secureboot

## Strategy

Currently all Secureboot capable systems are based on UEFI
Secureboot. Other types might be supported in the future.

Artefacts currently subject to secureboot verification currently are:
- systemd-boot.efi (provides bootloader menu, entry selection, etc)
- vmlinuz (linux kernel loaded from type 1 systemd-boot entries)

Other artefacts might be added in the future.

Currently we are trying to implement keyless PQC-safe Secureboot:
- only trust individual SHA256 hashes of the objects being booted
- no support for rotating these valus
- no support for any long-lived asymmetric keys

This removes need to manage any keys, provides most secure
implementation comptible with User Mode and Deployed Mode.

This also gives customers ultimate freedom to replace any components,
recalculate own hashes, without need to manage any signing keys.

## Signature list settings

### GUID

The Chainguard OS GUID for identifying all EFI signature lists we
produce, is self-assigned and shall be
`c04269ed-5f3b-46df-ac37-95e3e70e4027`. This is to help identify at
runtime the ESL lists that originated from us, on the end user
machines. There are no other human readable descriptions or comments.

### PK

Any non-null value, without any particular certificate, to indicate that SecureBoot is on, but there is no capability to rotate KEK.

Current implementation is an EFI Signature List, of type SHA256 hashes, with zero hashaes present.

### KEK

In theory KEK is optional. In practice many implementations require KEK to be set.

Current implementation is an EFI Signature List, of type SHA256 hashes, with zero hashaes present.

### db

Current implementation is an EFI Signature List, of type SHA256 hashes, with 2 hashes present.

The hashes are Authenticode SHA256 of systemd-boot and vmlinuz present in the image.

### dbx

In theory dbx is optional. In practice many implementations require dbx to be set.

Current implementation is an EFI Signature List, of type SHA256 hashes, with zero hashaes present.

## Delivery formats

### Self-enroll

Above EFI signature lists, have been converted to unsigned
authenticated variable updates (.auth) and placed in ESP under auto
enrollment directory.

Systemd-boot is set to automatically attempt to enroll these in VMs.

On bare metal systems, systemd-boot offers a menu to enroll these
keys.

EDK / OVMF / BIOS - depending on implementation may have user
interface to navigate ESP and enroll these as well.

Self-enrollment is tested by qemu runner booting with secureboot code
firmware and empty variables. Then self enrollment is performed. Then
secureboot test cases are executed to verify secureboot mode is on.

References:
- https://www.freedesktop.org/software/systemd/man/latest/systemd-boot.html#Files
- https://www.freedesktop.org/software/systemd/man/latest/loader.conf.html#secure-boot-enroll


### Qemu VARS

For x86_64 secureboot builds of EDK2 OVMF firmware, and aarch64
secureboot builds of EKS AAVMF firmware two uefi-vars.fd is provided
in a 4k / 64MB compatible formats.

These can be used together with CODE.fd to boot Qemu VMs directly in
secureboot mode.

This is tested by qemu runner booting with secureboot code firmware
and the per-VM-build-specific firmware variables. The secureboot test
cases are executed to verify secureboot mode is on.

### AWS Vars

For AWS, uefi-data.aws file is generated in the AWS specific format,
and then AMI is registered with said file.

### GCP Vars

GCP image registration has flags to set pk,kek,db,dbx.

When unset, default values are used, that default to Microsoft chain.

When set to filenames ending in `.bin` the file format is expected to
be a well-formed authenticated EFI variable update. Aka result of
`sign-efi-sig-list`. The key used for signing and the signatures are
actually unchecked. This is the same format as used for Qemu
self-enroll, but with more strict format validation.

For now these are generated with on-the-fly generated signing key. In
the future, if we get a long-lived ephemeral key, we could make all
our .auth files to be well formed.

As of October 2025, only db is getting succesfully enrolled, and stock
GCP (google/microsoft) is used for PK, KEK, dbx.

## Future work

Explore enrolling and trusting the new Microsoft Option ROM signing
key.

Explore how to enroll for Azure.

Explore secureboot signing on Raspberry Pi.

For immutable VMs explore creating UKI with root hash, and enrolling that.

Explore modifying systemd-boot to enrol multiple auth files, similar
to sbkeysync. And also consider packaging self-enrol files such that
they come from APKs at build time.

Ensure that empty PK, KEK, dbx is enrolled on GCP.
