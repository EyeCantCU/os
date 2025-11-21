#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

repo=$1

get_prs() {
  gh search prs --repo chainguard-dev/enterprise-packages --app "octo-sts" --label "approver-bot/approve" --limit=150 --review none --state open --json number --jq '.[].number'
}

readarray -t PRS < <(get_prs)
FAILED_PRS=()
for pr in "${PRS[@]}"; do
  echo ">>> Reviewing PR: ${pr}"

  # Approve PR
  if curl \
    -o review_output.json \
    -X POST \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GH_TOKEN}" \
    https://api.github.com/repos/"${repo}"/pulls/"${pr}"/reviews
  then
    echo "review: "
    cat review_output.json
    REVIEW_ID=$(jq -r '.id' review_output.json)
    GITHUB_TOKEN="${GH_TOKEN}" gh api \
      --method POST \
      -H "Accept: application/vnd.github+json" \
      /repos/"${repo}"/pulls/"${pr}"/reviews/"${REVIEW_ID}"/events \
      -f event='APPROVE'
  else
    FAILED_PRS+=("${pr}")
  fi
done

if [ ${#FAILED_PRS[@]} -ne 0 ]; then
  echo "The following PRs failed to be approved:"
  for pr in "${FAILED_PRS[@]}"; do
    echo "\t${pr}"
  done
  exit 1
fi
