#!/usr/bin/env bash
set -eux -o pipefail

# Enter repo root
cd "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../"

EXTRA_REPOS=(
    "https://packages.cgr.dev/extras" # contains "mattmoor-chainit-init"
)

EXTRA_KEYRINGS=(
    "https://packages.cgr.dev/extras/chainguard-extras.rsa.pub"
)

EXTRA_PKGS=(
    "linux-boot-configuration"
    "mattmoor-chainit-init"
)

mkdir -p converted/minirootfs/

# Create apko YAMLs that are modified to work in VM land
for entry in $(grep -v '\#' convert.txt); do
    cosign verify-attestation \
        --type="https://apko.dev/image-configuration" \
        --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
        --certificate-identity-regexp="https://github.com/chainguard-images/images-private/.github/workflows/.*.yaml@refs/heads/main" \
        $entry > att.json
    jq -r . att.json | yq -P > att.tmp && rm att.json
    yq -P -i '.payload | @base64d | fromjson | .predicate' att.tmp
    for i in "${EXTRA_REPOS[@]}"; do \
        yq -i ".contents.repositories += [\"$i\"]" att.tmp; \
    done
    for i in "${EXTRA_KEYRINGS[@]}"; do \
        yq -i ".contents.keyring += [\"$i\"]" att.tmp; \
    done
    for i in "${EXTRA_PKGS[@]}"; do \
        yq -i ".contents.packages += [\"$i\"]" att.tmp; \
    done
    yaml_filename="$(echo $entry | sed 's|/|_|g').yaml"
    mv att.tmp "converted/$yaml_filename"
done

# For each converted YAML file, build mini rootfs tar.gz files
for entry in $(grep -v '\#' convert.txt); do
    yaml_filename="$(echo $entry | sed 's|/|_|g').yaml"
    tgz_filename="$(echo $entry | sed 's|/|_|g').tar.gz"
    apko build-minirootfs "converted/$yaml_filename" "converted/minirootfs/$tgz_filename"
done

# TODO: publish the tar.gz files somewhere special
