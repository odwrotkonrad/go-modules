#!/usr/bin/env zsh
##[>] 🤖🤖
# Create a GitLab release + git tag for a branch's commit.
# Arg $1 = branch name, arg $2 = commit ref (SHA) to tag.
# Computes the tag via next-tag.zsh, then calls release-cli. A tag
# containing '-' is a prerelease and gets a prerelease description.
set -eu

branch="${1:?branch required}"
ref="${2:?ref required}"

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
