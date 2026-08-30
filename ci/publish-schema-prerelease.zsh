#!/usr/bin/env zsh
##[>] 🤖🤖
# MR-pipeline prerelease publish for every schema. Uploads the freshly generated
# che.schema.json and packages.schema.json to the generic package registry at
# packages/generic/<name>-schema/0.0.0-mr<iid>/<name>.schema.json, so an unmerged
# vocabulary change is validatable by che-packages before that che ships.
#
# The catalog schema publishes as che-packages, one name carrying the repo it
# describes, while docgen still generates it as packages.schema.json.
#
# The short 0.0.0-mr<iid> form, never -che-min-<version>: SemVer allows one
# prerelease part, and a floor is meaningless for a schema whose only pairing is
# that same MR's che prerelease.
#
# Its own job, not a step of prerelease-{linux,darwin}-che: a schema is
# arch-independent and must upload exactly once, while those run per os/arch.
set -eu

VERSION="0.0.0-mr${CI_MERGE_REQUEST_IID:?}"

for schema in che che-packages che-variables; do
  DOC="${schema}.schema.json"
  FILE="che/assets/data/${${schema#che-}}.schema.json"
  [[ -f "$FILE" ]] || { print -u2 "publish-schema-prerelease: ${FILE} not generated"; exit 1 }

  echo "uploading ${FILE:t} -> ${schema}-schema/${VERSION}/${DOC}"
  curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --upload-file "$FILE" \
    "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${schema}-schema/${VERSION}/${DOC}"
  echo
done

echo "schema prereleases published: CHE_SCHEMA_REF=${VERSION} CHE_PACKAGES_SCHEMA_REF=${VERSION} CHE_VARIABLES_SCHEMA_REF=${VERSION}"
##[<] 🤖🤖
