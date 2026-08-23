#!/usr/bin/env zsh
##[>] 🤖🤖
# Compute the version a schema should publish at, and whether it changed.
# Args: $1 schema (che|che-packages), $2 the che version this release cuts at
# (v<major>.<minor>.<patch>, the che-min floor; unused on a merge request).
#
# On a merge-request pipeline prints 0.0.0-mr<iid> and exits 2: a prerelease
# republishes on every push, so the changed check does not apply to it.
#
# Otherwise finds the newest published <schema>-schema/v<triple>-che-min-v<triple>
# tag, downloads that version's schema document from the generic package
# registry and compares it byte for byte with the freshly generated one. Prints
# the existing version and exits 1 when identical (nothing to mint), else prints
# the patch-bumped version carrying -che-min-<che version> and exits 0.
#
# Exit code is the answer, stdout is the version: 0 mint, 1 unchanged, 2 prerelease.
set -eu

SCHEMA="${1:?schema (che|che-packages)}"
CHE_VERSION="${2:-}"

NAME="${SCHEMA}-schema"
DOC="${SCHEMA}.schema.json"
#[why] the published name and the generated filename differ for the catalog schema: docgen writes
#   assets/data/packages.schema.json, which che's own code, the builtin packages.yml header and the
#   tests all name, while the artifact is published as che-packages so its identity carries the repo
GENERATED="che/assets/data/${${SCHEMA#che-}}.schema.json"

[[ -f "$GENERATED" ]] || { print -u2 "schema-version: ${GENERATED} not generated"; exit 3 }

fetch() {
  if (( $+commands[curl] )) {
    curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors --header "JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  } else {
    wget -qO- --header="JOB-TOKEN: ${CI_JOB_TOKEN}" "$1"
  }
}

if [[ "${CI_PIPELINE_SOURCE:-}" == merge_request_event ]] {
  print "0.0.0-mr${CI_MERGE_REQUEST_IID:?}"
  exit 2
}

[[ -n "$CHE_VERSION" ]] || { print -u2 'schema-version: che version required off a merge request'; exit 3 }

#[why] sort on the leading triple alone: -che-min-v<triple> is a prerelease segment carrying a
#   floor, never a precedence input, and two schema versions never share a triple
LATEST=$(
  fetch "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/repository/tags?per_page=100&order_by=version&sort=desc&search=^${NAME}/v" \
    | grep -oE "\"name\":\"${NAME}/v[0-9]+\.[0-9]+\.[0-9]+-che-min-v[0-9]+\.[0-9]+\.[0-9]+\"" \
    | sed -E "s|.*\"${NAME}/v(.*)\"|\1|" \
    | sort -t. -k1,1n -k2,2n -k3,3n \
    | tail -1
)
[[ -z "$LATEST" ]] || LATEST="v${LATEST}"

if [[ -z "$LATEST" ]] {
  print "v0.0.1-che-min-${CHE_VERSION}"
  exit 0
}

#[why] a tagged-but-unpublished version must mint rather than fail: the comparison has nothing to
#   compare against, and treating that as "unchanged" would leave the document published nowhere
PUBLISHED=$(mktemp)
trap "rm -f ${(q)PUBLISHED}" EXIT

fetch_published() {
  fetch "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${NAME}/${LATEST}/${DOC}" > "$PUBLISHED" 2> /dev/null
}

if { ! fetch_published } {
  print -u2 "schema-version: ${NAME}/${LATEST} is tagged but its document is not published, minting"
} elif { cmp -s "$PUBLISHED" "$GENERATED" } {
  print "$LATEST"
  exit 1
}

TRIPLE="${${LATEST#v}%%-*}"
MAJOR="${TRIPLE%%.*}"
REST="${TRIPLE#*.}"
MINOR="${REST%%.*}"
PATCH="${REST#*.}"
print "v${MAJOR}.${MINOR}.$((PATCH + 1))-che-min-${CHE_VERSION}"
##[<] 🤖🤖
