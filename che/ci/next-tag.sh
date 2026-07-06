#!/bin/sh
##[>] 🤖🤖
# Compute the next release tag for a branch.
# Arg $1 = branch name. On the default branch prints the next stable
# v<major>.<minor>.<patch> (highest vX.Y.Z tag, patch-bumped). On any
# other branch prints a prerelease v<target>-<branch>.<N>, where <branch>
# is sanitized to the SemVer prerelease charset [0-9A-Za-z-] and N =
# highest existing v<target>-<branch>.* counter + 1 (else 1).
# Prints the tag to stdout.
set -eu

branch="${1:-}"
default="${CI_DEFAULT_BRANCH:-main}"

api="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/tags?per_page=100&order_by=version&sort=desc"

fetch() {
  wget -qO- --header="JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
}

# next stable target from highest vX.Y.Z tag (rc tags excluded by the grep)
latest=$(
  fetch "$api" \
    | grep -oE '"name":"v[0-9]+\.[0-9]+\.[0-9]+"' \
    | sed -E 's/.*"v([0-9]+\.[0-9]+\.[0-9]+)"/\1/' \
    | sort -t. -k1,1n -k2,2n -k3,3n \
    | tail -1
)

if [ -z "$latest" ]; then
  target="v0.0.1"
else
  major=$(echo "$latest" | cut -d. -f1)
  minor=$(echo "$latest" | cut -d. -f2)
  patch=$(echo "$latest" | cut -d. -f3)
  target="v${major}.${minor}.$((patch + 1))"
fi

if [ "$branch" = "$default" ]; then
  echo "$target"
  exit 0
fi

# prerelease: sanitize branch to SemVer charset, bump v<target>-<branch>.<N>
slug=$(echo "$branch" | sed -E 's/[^0-9A-Za-z-]+/-/g; s/^-+//; s/-+$//')
pre="${target}-${slug}"

esc=$(echo "$pre" | sed 's/\./\\./g')
max=$(
  fetch "$api" \
    | grep -oE "\"name\":\"${esc}\.[0-9]+\"" \
    | sed -E 's/.*\.([0-9]+)"/\1/' \
    | sort -n \
    | tail -1
)

echo "${pre}.$(( ${max:-0} + 1 ))"
##[<] 🤖🤖
