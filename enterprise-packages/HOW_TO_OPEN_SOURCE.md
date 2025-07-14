# How to Open Source a Package

Sometimes packages may end up in this repository when they should actually land in [wolfi-dev/os](https://github.com/wolfi-dev/os).

If that is the case then here is the process to follow to open source the package

1. Ensure the LICENSE meets the Wolfi standard (TODO: Add a link to acceptance critera for this repo when it exists)
1. Copy the contents of the melange config into the [wolfi-dev/os](https://github.com/wolfi-dev/os) repo
1. Copy any of the patches into the [wolfi-dev/os](https://github.com/wolfi-dev/os) repo
1. Bump the epoch of the melange config to make sure the Wolfi version of the package will be the latest
1. Submit PR in [wolfi-dev/os](https://github.com/wolfi-dev/os)
1. Once the PR is merged wait for the package to be built (check with `wolfictl apk ${pkg}-${version}-r${epoch}.apk`)
1. Remove the melange config and any patches from this repository

**Note:** The packages themselves do not need to be withdrawn unless they are truly broken. They can be left in the GCP bucket.

At this point you should have the melange config living in [wolfi-dev/os](https://github.com/wolfi-dev/os) and the config deleted
out of this repo. The package at https://packages.wolfi.dev should be the latest version available.
