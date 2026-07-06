#!/usr/bin/env zsh
##[>] 🤖🤖
# Create a GitLab release + git tag for a branch's commit.
# Env BRANCH = branch name, REF = commit ref (SHA) to tag.
# Computes the tag via next-tag.zsh, then calls release-cli. A tag
# containing '-' is a prerelease and gets a prerelease description.
set -eu

branch="${BRANCH:?BRANCH required}"
ref="${REF:?REF required}"

dir="${0:a:h}"
tag=$("$dir/next-tag.zsh" "$branch")

if [[ "$tag" == *-* ]]; then
  desc="che prerelease $tag ($branch)"
else
  desc="che $tag"
fi

echo "tagging $tag at $ref"
release-cli create \
  --name "che $tag" \
  --tag-name "$tag" \
  --ref "$ref" \
  --description "$desc"
##[<] 🤖🤖
