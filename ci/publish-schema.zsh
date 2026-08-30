#!/usr/bin/env zsh
##[>] 🤖🤖
# Publish one schema document at a version, and link it to that version's
# release. Args: $1 schema (che|che-packages|che-variables), $2 version
# (v<triple>-che-min-v<triple>).
#
# Uploads to packages/generic/<schema>-schema/<version>/<schema>.schema.json,
# then re-uploads to a moving latest/ alias. The versioned path is what a
# consumer pins; the alias is what a reader with no version reaches for.
set -eu

SCHEMA="${1:?schema (che|che-packages|che-variables)}"
VERSION="${2:?version}"

NAME="${SCHEMA}-schema"
DOC="${SCHEMA}.schema.json"
#[why] the catalog schema publishes as che-packages but is generated as packages.schema.json
FILE="che/assets/data/${${SCHEMA#che-}}.schema.json"

[[ -f "$FILE" ]] || { print -u2 "publish-schema: ${FILE} not generated"; exit 1 }

PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${NAME}"
TAG="${NAME}/${VERSION}"
TAG_ENC="${TAG//\//%2F}"

echo "uploading ${DOC} -> ${NAME}/${VERSION}"
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
  --upload-file "$FILE" "${PKG}/${VERSION}/${DOC}"
echo

curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --request POST --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
  --data-urlencode "name=${DOC}" \
  --data-urlencode "url=${PKG}/${VERSION}/${DOC}" \
  --data-urlencode "link_type=other" \
  "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases/${TAG_ENC}/assets/links"
echo

echo "aliasing ${DOC} -> ${NAME}/latest"
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
  --upload-file "$FILE" "${PKG}/latest/${DOC}"
echo
##[<] 🤖🤖
