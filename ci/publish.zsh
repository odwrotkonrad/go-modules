#!/usr/bin/env zsh
##[>] 🤖🤖
# Tag-pipeline publish for one module. Derives MODULE + MODULE_VERSION from
# $CI_COMMIT_TAG (<module>/v<version>), builds with goreleaser in snapshot mode
# (build engine only: free goreleaser cannot parse dir-prefixed tags) unless
# PUBLISH_PREBUILT=1 (dist/*.tar.gz prebuilt by earlier jobs: checksums only),
# uploads
# every archive + checksums (che: plus a docs-site tarball from the docs-che
# job's public/ artifact) to the generic package registry at
# packages/generic/<module>/<version>/<file> and links each upload as a release
# asset on the existing <module>/v<version> release.
set -eu

TAG="${CI_COMMIT_TAG:?}"
MODULE="${TAG%%/v*}"
MODULE_VERSION="${TAG#*/v}"

cd "$MODULE"
if [[ "$MODULE" == che && -d ../public ]] {
  mkdir -p dist
  tar -czf "dist/che-docs_${MODULE_VERSION}.tar.gz" -C ../public .
}
if [[ "${PUBLISH_PREBUILT:-0}" == 1 ]] {
  ( cd dist && sha256sum -- *.tar.gz > checksums.txt )
} else {
  MODULE_VERSION="$MODULE_VERSION" goreleaser release --verbose --snapshot --clean -f .goreleaser.yaml
}

typeset -a FILES
FILES=(dist/*.tar.gz(N) dist/checksums.txt(N) darwin-dist/*.tar.gz(N))
if [[ "$MODULE" == che ]] FILES+=(assets/data/che.schema.json)

PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/${MODULE_VERSION}"
TAG_ENC="${TAG//\//%2F}"

for f in $FILES; do
  NAME="${f:t}"
  echo "uploading ${NAME}"
  curl -fsSL --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "$f" "${PKG}/${NAME}"
  echo
  curl -fsSL --request POST --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --data-urlencode "name=${NAME}" \
    --data-urlencode "url=${PKG}/${NAME}" \
    "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases/${TAG_ENC}/assets/links"
  echo
done
##[<] 🤖🤖
