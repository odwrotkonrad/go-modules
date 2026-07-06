#!/bin/sh
##[>] 🤖🤖
# Compute the next SemVer prerelease tag for the current MR commit.
# Reuses next-version.sh for the target stable, then appends -rc.<N>
# where N = highest existing v<target>-rc.* counter + 1 (else 1).
# Prints the tag (e.g. v0.0.7-rc.1) to stdout.
set -eu

dir=$(dirname "$0")
target=$("$dir/next-version.sh")

api="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/tags?per_page=100&order_by=version&sort=desc"

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --header "JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  else
    wget -qO- --header="JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  fi
}

esc=$(echo "$target" | sed 's/\./\\./g')
max=$(
  fetch "$api" \
    | grep -oE "\"name\":\"${esc}-rc\.[0-9]+\"" \
    | sed -E 's/.*-rc\.([0-9]+)"/\1/' \
    | sort -n \
    | tail -1
)

echo "${target}-rc.$(( ${max:-0} + 1 ))"
##[<] 🤖🤖
