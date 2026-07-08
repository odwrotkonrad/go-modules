#!/usr/bin/env zsh
##[>] 🤖🤖
# Compute the next release tag for a module on a branch.
# Args: $1 module, $2 branch (default: current), $3 default branch (default: main).
# On the default branch prints the next stable <module>/v<major>.<minor>.<patch>
# (highest <module>/vX.Y.Z tag, patch-bumped). On any other branch prints a
# prerelease <module>/v<target>-<branch>.<N>, where <branch> is sanitized to the
# SemVer prerelease charset [0-9A-Za-z-] and N = highest existing
# <module>/v<target>-<branch>.* counter + 1 (else 1). Prints the tag to stdout.
set -eu

MODULE="$1"
BRANCH="${2:-$(git rev-parse --abbrev-ref HEAD)}"
DEFAULT="${3:-main}"

API="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/tags?per_page=100&order_by=version&sort=desc&search=^${MODULE}/v"

fetch() {
  if (( $+commands[curl] )) {
    curl -fsSL --header "JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  } else {
    wget -qO- --header="JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  }
}

# next stable target from highest <module>/vX.Y.Z tag (rc tags excluded by the grep)
LATEST=$(
  fetch "$API" \
    | grep -oE "\"name\":\"${MODULE}/v[0-9]+\.[0-9]+\.[0-9]+\"" \
    | sed -E "s|.*\"${MODULE}/v([0-9]+\.[0-9]+\.[0-9]+)\"|\1|" \
    | sort -t. -k1,1n -k2,2n -k3,3n \
    | tail -1
)

if [[ -z "$LATEST" ]]; then
  TARGET="v0.0.1"
else
  MAJOR=${LATEST%%.*}
  REST=${LATEST#*.}
  MINOR=${REST%%.*}
  PATCH=${REST#*.}
  TARGET="v${MAJOR}.${MINOR}.$((PATCH + 1))"
fi

if [[ "$BRANCH" == "$DEFAULT" ]]; then
  echo "${MODULE}/${TARGET}"
  exit 0
fi

# prerelease: sanitize branch to SemVer charset, bump <module>/v<target>-<branch>.<N>
SLUG=$(echo "$BRANCH" | sed -E 's/[^0-9A-Za-z-]+/-/g; s/^-+//; s/-+$//')
PRE="${TARGET}-${SLUG}"

ESC=$(echo "${MODULE}/${PRE}" | sed 's/\./\\./g')
MAX=$(
  fetch "$API" \
    | grep -oE "\"name\":\"${ESC}\.[0-9]+\"" \
    | sed -E 's/.*\.([0-9]+)"/\1/' \
    | sort -n \
    | tail -1
)

echo "${MODULE}/${PRE}.$(( ${MAX:-0} + 1 ))"
##[<] 🤖🤖
