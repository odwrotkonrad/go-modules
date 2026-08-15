#!/usr/bin/env zsh
##[>] 🤖🤖
# Tag-pipeline publish for the data-only che-packages module. Derives the
# version from $CI_COMMIT_TAG (che-packages/v<version>), tars packages.yml +
# scripts/ into che-packages_<version>.tar.gz with a sha256 checksums.txt,
# uploads both to the generic package registry at
# packages/generic/che-packages/<version>/ and links them as release assets,
# then refreshes the moving latest/ alias: a de-versioned tarball plus
# version.txt, which `che packages update` reads to resolve the newest version.
set -eu

TAG="${CI_COMMIT_TAG:?}"
MODULE="${TAG%%/v*}"
MODULE_VERSION="${TAG#*/v}"

cd "$MODULE"
NAME="${MODULE}_${MODULE_VERSION}.tar.gz"
mkdir -p dist
tar -czf "dist/${NAME}" packages.yml scripts
( cd dist && sha256sum -- "$NAME" > checksums.txt )

PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/${MODULE_VERSION}"
TAG_ENC="${TAG//\//%2F}"

for f in "dist/${NAME}" dist/checksums.txt; do
  FILE_NAME="${f:t}"
  TYPE=other
  if [[ "$FILE_NAME" == *.tar.gz ]] TYPE=package
  echo "uploading ${FILE_NAME} (${TYPE})"
  curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "$f" "${PKG}/${FILE_NAME}"
  echo
  curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --request POST --header "JOB-TOKEN: ${CI_JOB_TOKEN}" \
    --data-urlencode "name=${FILE_NAME}" \
    --data-urlencode "url=${PKG}/${FILE_NAME}" \
    --data-urlencode "link_type=${TYPE}" \
    "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/releases/${TAG_ENC}/assets/links"
  echo
done

ALIAS="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/latest"
echo "aliasing ${NAME} -> latest"
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "dist/${NAME}" "${ALIAS}/${MODULE}_latest.tar.gz"
echo
print -r -- "$MODULE_VERSION" > dist/version.txt
echo "uploading latest/version.txt (${MODULE_VERSION})"
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file dist/version.txt "${ALIAS}/version.txt"
echo
##[<] 🤖🤖
