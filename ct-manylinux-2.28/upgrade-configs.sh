#!/bin/sh

set -e

# Must run this in the package config directory
[ "$(basename $(pwd))" = "ct-manylinux-2.28" ]

pkgcfgdir="$(pwd)"

sudo apk add crosstool-ng yq
tmpdir="$(mktemp -d)"
trap 'rm -rf $tmpdir' EXIT

mkdir -p "$tmpdir/src"

export CT_PREFIX="$tmpdir"

for config in "$pkgcfgdir"/config.*; do
    cp "$config" "$tmpdir"/.config
    cd "$tmpdir"
    ct-ng upgradeconfig
    cp .config "$config"
    sed -i 's/^CT_FORBID_DOWNLOAD=y/# CT_FORBID_DOWNLOAD is not set/' .config
    sed -i '/^CT_LOCAL_TARBALLS_DIR=/d' .config
    echo "CT_LOCAL_TARBALLS_DIR=$tmpdir/src" >> .config
    ct-ng source
    cd -
    for uri in $(yq -r '.pipeline[] | select(.uses) | .with.uri' < ../ct-manylinux-2.28.yaml); do
        tarball="$(basename "${uri}")"
        test -f "$tmpdir"/src/"${tarball}" || echo "WARNING: ${tarball} not found. It has likely been bumped to a new minor version and requires an update to its fetch pipeline" 1>&2
    done
done

    

