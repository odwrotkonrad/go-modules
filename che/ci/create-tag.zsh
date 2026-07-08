#!/usr/bin/env zsh
##[>] 🤖🤖
# Create a GitLab release + git tag for a branch's commit.
# Args: $1 branch (default: current), $2 ref/SHA (default: HEAD),
# $3 default branch (default: main). Computes the tag via next-tag.zsh,
# then calls glab release create. A tag containing '-' is a prerelease and gets
# a prerelease description.
set -eu

BRANCH="${1:-$(git rev-parse --abbrev-ref HEAD)}"
REF="${2:-$(git rev-parse HEAD)}"
DEFAULT="${3:-main}"

ROOT=$(git rev-parse --show-toplevel)
TAG=$("$ROOT/ci/next-tag.zsh" "$BRANCH" "$DEFAULT")

if [[ "$TAG" == *-* ]]; then
  DESC="che prerelease $TAG ($BRANCH)"
else
  DESC="che $TAG"
fi

echo "tagging $TAG at $REF"
glab release create "$TAG" \
  --name "che $TAG" \
  --ref "$REF" \
  --notes "$DESC"
##[<] 🤖🤖
