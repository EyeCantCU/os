#!/usr/bin/env bash

git submodule

# If there are any changes, proceed
if [[ -n $(git status --porcelain) ]]; then
    git checkout -b sync-submodules
    git commit -asm "update submodules"
    git push origin sync-submodules --force

    # If PR exists for this branch, update it
    # Get PR number for this branch
    pr_number=$(gh pr list --state open --base main --head sync-submodules | grep sync-submodules | awk '{print $1}')
    if [[ -n $pr_number ]]; then
        gh pr update $pr_number
    else
        gh pr create --title "update submodules" --body "update submodules" --base main
    fi
    git checkout -
else
    echo "No changes to submodules, nothing to do"
fi

