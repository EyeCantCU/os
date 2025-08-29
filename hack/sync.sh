#!/usr/bin/env bash

git subtree pull -m "sync wolfi" --prefix os https://github.com/wolfi-dev/os main
git subtree pull -m "sync extras" --prefix extra-packages https://github.com/chainguard-dev/extra-packages main
git subtree pull -m "sync enterprise" --prefix enterprise-packages https://github.com/chainguard-dev/enterprise-packages main
git subtree pull -m "sync vms" --prefix wolfi-vm https://github.com/chainguard-dev/wolfi-vm main
