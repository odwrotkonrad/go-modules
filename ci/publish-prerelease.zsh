#!/usr/bin/env zsh
##[>] 🤖🤖
# MR-pipeline prerelease publish for che. Uploads this job's che archives to the
# generic package registry at packages/generic/che/0.0.0-mr<iid>/<file>, so an
# unmerged che change installs through the published install.sh with
# CHE_VERSION=0.0.0-mr<iid>. Runs as a final step of prerelease-{linux,darwin}-che:
# each job passes its own dist dir and uploads only its own os/arch archives, so
# the two never collide on a filename. Keyed on the MR iid, never the commit: a
# later push to the same MR overwrites these files, one prerelease per MR. No
# release-asset links (an MR has no release) and no latest/ alias touch (that
# alias belongs to real releases).
set -eu

DIST_DIR="${1:?dist dir}"
MODULE_VERSION="${MODULE_VERSION:?}"
MODULE=che

cd "$MODULE"

typeset -a FILES
FILES=(${DIST_DIR}/${MODULE}_${MODULE_VERSION}_*.tar.gz(N))
if (( ! ${#FILES} )) {
  print -u2 "publish-prerelease: no ${MODULE}_${MODULE_VERSION}_*.tar.gz in ${MODULE}/${DIST_DIR}"
  exit 1
}

PKG="${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/packages/generic/${MODULE}/${MODULE_VERSION}"

for f in $FILES; do
  NAME="${f:t}"
  echo "uploading ${NAME} -> ${MODULE}/${MODULE_VERSION}"
  curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --header "JOB-TOKEN: ${CI_JOB_TOKEN}" --upload-file "$f" "${PKG}/${NAME}"
  echo
done

echo "prerelease published: CHE_VERSION=${MODULE_VERSION}"
##[<] 🤖🤖
