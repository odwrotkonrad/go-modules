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
# asset on the existing <module>/v<version> release, then re-uploads each os/
# arch archive to a moving packages/generic/<module>/latest/ alias (de-versioned
# filename, no release-asset link) so consumers fetch latest with no version.
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
if [[ -d darwin-dist ]] ( cd darwin-dist && sha256sum -- *.tar.gz >> ../dist/checksums.txt )

typeset -a FILES
FILES=(
  dist/${MODULE}_*.tar.gz(N) darwin-dist/${MODULE}_*.tar.gz(N)
  dist/render-*.tar.gz(N) darwin-dist/render-*.tar.gz(N)
  dist/checksums.txt(N)
)
if [[ "$MODULE" == che ]] FILES+=(assets/data/che.schema.json dist/che-docs_*.tar.gz(N))

PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/${MODULE_VERSION}"
TAG_ENC="${TAG//\//%2F}"

for f in $FILES; do
  NAME="${f:t}"
  TYPE=other
  if [[ "$NAME" == *.tar.gz && "$NAME" != che-docs_* ]] TYPE=package
  echo "uploading ${NAME} (${TYPE})"
  curl -fsSL --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "$f" "${PKG}/${NAME}"
  echo
  curl -fsSL --request POST --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --data-urlencode "name=${NAME}" \
    --data-urlencode "url=${PKG}/${NAME}" \
    --data-urlencode "link_type=${TYPE}" \
    "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases/${TAG_ENC}/assets/links"
  echo
done

#[why] re-upload each os/arch archive to a de-versioned latest/ path, a moving alias so consumers fetch <module>/latest/<module>_latest_<os>_<arch>.tar.gz with no version, no checksum; versioned paths + release assets untouched
ALIAS="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/latest"
for f in dist/${MODULE}_${MODULE_VERSION}_linux_*.tar.gz(N) darwin-dist/${MODULE}_${MODULE_VERSION}_darwin_*.tar.gz(N); do
  plat="${${f:t:r:r}#${MODULE}_${MODULE_VERSION}_}"
  echo "aliasing ${MODULE}_${plat} -> latest"
  curl -fsSL --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "$f" "${ALIAS}/${MODULE}_latest_${plat}.tar.gz"
  echo
done
##[<] 🤖🤖
