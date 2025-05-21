#!/usr/bin/bash

### From https://github.com/kranurag7/mono/blob/485e96e484f08ca5ab81fc54e55c34232e1db67b/env/enforce.dev/iac/100-prereqs/main.tf
readonly cg_aud="${CHAINGUARD_AUDIENCE:-"https://issuer.enforce.dev"}"

function get_chainguard_identity_from_metadata() {
    curl "http://metadata.google.internal/computeMetadata/v1/project/attributes/ChainguardIdentity" -H "Metadata-Flavor: Google"
}

function get_identity_token() {
    gcloud auth print-identity-token --audiences="${cg_aud}"
}

function main() {
    # If somebody has run this workstation with CHAINGUARD_IDENTITY=""
    # let them know but do not crash
    echo "Info: Configuring chainctl, please wait..."
    local readonly identity="$(get_chainguard_identity_from_metadata)"
    if [[ "${identity}" == "" ]]; then
        echo "Warning: Chainguard Identity not found. Skipping cgr setup."
        exit 0
    fi

    local readonly token="$(get_identity_token)"
    echo "Info: Authenticating with chainctl..."
    chainctl auth login \
        --identity="${identity}" --identity-token="${token}"

    echo "Info: Authenticating with apk.cgr.dev..."
    chainctl auth login \
        --identity="${identity}" --identity-token="${token}" --audience=apk.cgr.dev

    # Note: we do this symlink ourselves (vs. in configure-docker)
    # since chainctl is owned by root
    sudo ln -sf /usr/bin/chainctl /usr/bin/docker-credential-cgr

    echo "Info: Configure docker chainctl authentication..."
    chainctl auth configure-docker \
        --identity="${identity}" --identity-token="${token}"
}

main
