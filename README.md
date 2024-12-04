![Chainguard logo](https://avatars.githubusercontent.com/u/87436699?s=200&v=4)

# Chainguard Enterprise Wolfi Packages

This package repository will host all paid packages support by Chainguard but not made available
on [Wolfi](https://wolfi.dev/os).

New package requests are usually submitted as part of a customer request. Please visit
our '[Customer Engagements'](https://wiki.inky.wtf/docs/teams/engineering/images/customer-engagements/) page for more
details.

This repo is owned by the [Images Team](https://wiki.inky.wtf/docs/teams/engineering/images/chainguard-images/).

For help with patching CVEs and recording the associated advisory data, see the ["How To Patch CVEs"](https://github.com/wolfi-dev/os/blob/main/HOW_TO_PATCH_CVES.md) documentation in Wolfi.

# Contents

This repository is based on the open source [Wolfi OS repository](https://github.com/wolfi-dev/os) with a few differences.

### Add a new package

Tp add a new melange package, create a melange yaml file and make sure you add an entry to the Makefile that matches the `package.name` and `package.version`.

The main difference to Wolfi OS is that we need to specifiy the Wolfi OS repository and keyring so we can fetch dependencies when building.

i.e.

```yaml
environment:
  contents:
    repositories:
      - https://packages.wolfi.dev/os
    keyring:
      - https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
```

Because of this difference, when validating lint locally you need to override two default rules...

```sh
wolfictl lint --skip-rule forbidden-repository-used --skip-rule forbidden-keyring-used
```
