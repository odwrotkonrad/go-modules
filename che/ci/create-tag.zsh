#!/usr/bin/env zsh
##[>] 🤖🤖
# Create a GitLab release + git tag for a branch's commit.
# Args: $1 branch (default: current), $2 ref/SHA (default: HEAD),
# $3 default branch (default: main). Computes the tag via next-tag.zsh,
# then calls release-cli. A tag containing '-' is a prerelease and gets
# a prerelease description.
set -eu

branch="${1:-$(git rev-parse --abbrev-ref HEAD)}"
ref="${2:-$(git rev-parse HEAD)}"
default="${3:-main}"

dir="${0:a:h}"
tag=$("$dir/next-tag.zsh" "$branch" "$default")

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
