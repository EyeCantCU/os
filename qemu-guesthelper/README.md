# qemu-guesthelper
The goal of this is to be a little "guest helper" that will
facilitate host-to-guest operations.


## fetch-keys
The fetch-keys subcommand takes a user to fetch keys for, and writes keys it
finds to stdout.  This makes it useful to be called from an ssh `AuthorizedKeysCommand`.

The `qemu-guesthelper-authorized-keys-command` subpackage will install
a sshd config snippet into `/etc/ssh/sshd_config.d` that will call `fetch-keys`
when an ssh connection is made.

In order to provide keys to qemu-guest, 2 things must be set.

 1. the smbios 'product' must be set to 'cgr.dev/qemu/v1'
 2. there must be an OEM string in the smbios starting with `cgr.dev/qemu/v1/ssh-pubkey=`

In order to do this, you can invoke qemu like the following:

    $ pubkey=$(cat ~/.ssh/id_ed25519.pub)
    $ qemu-system-x86_64 -machine q35 -m 4G ... \
       -smbios "type=1,product=cgr.dev/qemu/v1" \
       -smbios "type=11,value=cgr.dev/qemu/v1/ssh-pubkey=$pubkey"


If the provided pubkey is of the format `<user>:<key>`, then the key will only
be valid for the provided user.  If the key does not contain a `:`, then it
is assumed to be valid for all users.
