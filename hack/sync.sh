#!/usr/bin/env bash

git subtree pull --prefix os https://github.com/wolfi-dev/os main
git subtree pull --prefix extra-packages https://github.com/chainguard-dev/extra-packages main
git subtree pull --prefix enterprise-packages https://github.com/chainguard-dev/enterprise-packages main
git subtree pull --prefix wolfi-vm https://github.com/chainguard-dev/wolfi-vm main
