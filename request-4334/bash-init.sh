#!/bin/bash

# Check if we are in the Chainguard Build Ecosystem
if [[ -n "${targets:-}" && -n "${targets.contextdir:-}" || "${TEST_PIPELINE}" == "true" ]]; then
    echo "Running inside the Chainguard Build Ecosystem"
else
    echo "https://artifactory.danskenet.net/artifactory/remote-alpine-cgr-extras" > /etc/apk/repositories
    echo "https://artifactory.danskenet.net/artifactory/remote-alpine-wolfi-packages" >> /etc/apk/repositories

    # Update-ca-certificates will cause an infinite bash loop if you force bash upon it when it thinks its invoking regular /bin/sh so making a temp symb link to busybox
    mv /bin/sh /bin/sh.bak
    ln -sf /bin/busybox /bin/sh
    update-ca-certificates
    rm /bin/sh
    mv /bin/sh.bak /bin/sh
fi
