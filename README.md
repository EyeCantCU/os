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
wolfitl lint --skip-rule forbidden-repository-used --skip-rule forbidden-keyring-used
```
---

__This repository also includes:__

## Git Submodules

The Enterprise package repository provides APK packages for paid Chainguard Images.  As a result there are certain cases where the source that is being compiled into APK comes from a private Git location, as an example Chainguard maintains a private fork of [Grafana](https://github.com/chainguard-dev/grafana/).

Today, melange does not support auth when fetching from private source Git Repositories.  As a tactical solution for now we are using Git Submodules to add a reference to the private Git repository, when our pipelines checkout the enterprise repo we include the submodules.  When the melange build runs the source is already available in the `--source-dir` for the package, i.e. `./grafana`.

### Adding a Git submodule

The Grafana Submodule was added using...

```sh
git submodule add --depth 1  https://github.com/chainguard-dev/grafana.git grafana
```

To add a new Submodule use the example above, ensuring the target directory matches the melange package name.

Once done make sure the `.gitmodules` file contains the correct repo and path.  It is good practice to also add a `branch:` field that matches a tag to the commit in the submodule.

### Auto updates

Using Git Submodules can cause some pain.  For this reason we have built some automation to keep these submodules up to date.  Continuing with the Grafana example, when a merge to the main branch is performed there is a GitHub Action that creates a new version based on previous Git Tags, creates a new GitHub Release and upgrades this Enterprise Git Repository via Pull Request.

[This is the GitHub Action](https://github.com/chainguard-dev/grafana/blob/2679925fb0a0a27e6ff4aef94fd011955f3e969c/.github/workflows/cg-release.yaml#L25-L44) that does this automation, built with `wolfictl`.  You should be able to copy the same Action into your new fork when adding another.

The `wolfictl update package` command will update the `.gitmodules` file as well as fetching the corresponding commit reference which in effect triggers an upgrade via automated GitHub Pull Request.

Additionally, when working on a fork, if you fix a CVE make sure to include a `fixes: CVE2023-1234` commit comment.  The generated Pull Request on the Enterprise repo will include auto generated advisory fix information and secfixes updates, for the fixed CVE.

### Manual updates

Modify `.gitmodules` to update the version

```
git submodule update
cd grafana
git pull
cd ..
git add | commit | push
```
