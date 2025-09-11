These base configs are loosely based on the Debian cloud kernel configs with excess things pulled out.

This structure removes system level config snippets in favor of a generic base for each arch and environment specific config (per cloud).

```
# Generic common configs
config-generic-x86_64
config-generic-aarch64

# Cloud specific additions
config-aws-generic
config-azure-generic
config-gcp-generic
config-qemu-generic
config-qemu-rc
config-vmware-generic

# Jitterentropy objects
config-jitterentropy
```

To update the base configs with new config option for new kernel versions you can largely do the following (TODO this can be automated)...

Install dependencies, recommend doing this on a Chainguard workstation. You need the following:
```
sudo chainctl auth login --audience apk.cgr.dev --headless
echo "https://apk.cgr.dev/chainguard-private" | sudo tee --append /etc/apk/repositories
sudo HTTP_AUTH="basic::user:$(chainctl auth token --audience apk.cgr.dev)" apk add pahole pahole-dev flex bison elfutils elfutils-dev diffutils jitterentropy-library-dev openssl-dev zstd
```

Checkout this repo (we'll assume we're going to do things in a directory called ~/src from here on)
```
mkdir src; cd src
# Or of course your fork
git checkout git@github.com:chainguard-dev/enterprise-packages.git
```

Checkout the gregkh linux repo
```
git checkout git@github.com:github.com/gregkh/linux.git
cd linux
```

Checkout the tag with the new base kernel (e.g. 6.16.2 base)
`git checkout tags/v6.16.2 -b v6.16.2`

Copy the common configs from this code base to the linux source
`cp ~/src/enterprise-packages/linux-common/config-generic* ~/src/linux`

Now we can use the make olddefconfig to automatically pull in new default config options.
```
cp config-generic-x86_64 .config
make olddefconfig # And then save the new config (default is to save to .config, that's fine).
cp .config config-generic-x86_64.new
```

For arm64 (on an x86_64 machine)
```
cp config-generic-aarch64 .config
ARCH=arm64 make olddefconfig # And then save the new config
cp .config config-generic-aarch64.new
```

You can now diff these and see what has changed (put this in the PR). You can use the diff or the kernel scripts
```
# using the kernel's kconfig diff script
scripts/diffconfig config-generic-aarch64 config-generic-aarch64.new
scripts/diffconfig config-generic-x86_64 config-generic-x86_64.new
# or diff
diff -y --suppress-common-lines config-generic-x86_64 config-generic-x86_64.new
diff -y --suppress-common-lines config-generic-aarch64 config-generic-aarch64.new
```

Finally, copy the new configs back to a branch in enterprise-packages to make a PR
```
# Make a new branch for your PR first...
cp config-generic-x86_64.new ~/src/enterprise-packages/linux-common/config-generic-x86_64
cp config-generic-aarch64.new ~/src/enterprise-packages/linux-common/config-generic-aarch64
```
