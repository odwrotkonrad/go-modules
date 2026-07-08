#!/usr/bin/env zsh
##[>] 🤖🤖
# Create a GitLab release + dir-prefixed git tag for a module at a commit.
# Args: $1 module, $2 branch (default: current), $3 ref/SHA (default: HEAD),
# $4 default branch (default: main). Computes the tag via next-tag.zsh, builds
# release notes from the module-scoped git log since the previous module tag,
# then calls glab release create. A tag containing '-' is a prerelease.
set -eu

MODULE="$1"
BRANCH="${2:-$(git rev-parse --abbrev-ref HEAD)}"
REF="${3:-$(git rev-parse HEAD)}"
DEFAULT="${4:-main}"

ROOT=$(git rev-parse --show-toplevel)
TAG=$("$ROOT/ci/next-tag.zsh" "$MODULE" "$BRANCH" "$DEFAULT")
VERSION="${TAG#${MODULE}/}"

PREV=$(git tag --list "${MODULE}/v*" --sort=-version:refname | grep -E "^${MODULE}/v[0-9]+\.[0-9]+\.[0-9]+$" | head -1 || true)
if [[ -n "$PREV" ]]; then
  NOTES=$(git log --no-merges --format='- %s' "${PREV}..${REF}" -- "${MODULE}/" 2>/dev/null || true)
else
  NOTES=$(git log --no-merges --format='- %s' "${REF}" -- "${MODULE}/" 2>/dev/null || true)
fi
if [[ "$TAG" == *-* ]]; then
  HEADER="${MODULE} prerelease ${VERSION} (${BRANCH})"
else
  HEADER="${MODULE} ${VERSION}"
fi
NOTES="${HEADER}"$'\n\n'"${NOTES:-}"

echo "tagging $TAG at $REF"
glab release create "$TAG" \
  --name "${MODULE} ${VERSION}" \
  --ref "$REF" \
  --notes "$NOTES"
##[<] 🤖🤖
