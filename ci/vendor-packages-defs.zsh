#!/usr/bin/env zsh
##[>] 🤖🤖
# Vendor the che-packages catalog che embeds. Reads the pinned version from
# $GRP_VAR_CHE_PACKAGES_REF, a group-wide CI variable raised by a catalog
# release, falling back to che's own $CHE_PACKAGES_REF for a local run,
# downloads that release's tarball from the che-packages generic package
# registry, verifies its sha256 against the published checksums.txt, and unpacks
# packages.yml + scripts/ into che/internal/packages/builtin/ (gitignored,
# rebuilt on demand). Re-running with an unchanged pin is a no-op.
set -eu
#[why] the user zshrc aliases rm to trash, which errors on a missing path: a
#   non-interactive run must reach the real binaries
emulate -L zsh

REPO="${0:A:h:h}"
DEST="${REPO}/che/internal/packages/builtin/data"
PROJECT="${CHE_PACKAGES_PROJECT:-konradodwrot%2Fche-packages}"

#[why] a group CI variable, not a file in che/: a pin file inside the module matched
#   release-che's `changes: [che/**/*]` rule, so raising it cut a che release carrying no che
#   change. a variable matches no path, releases nothing, and is the same name che's own
#   packages.source.ref reads, so CI and a config file pin through one mechanism
BASE="${CHE_PACKAGES_URL:-https://gitlab.com/api/v4/projects/${PROJECT}/packages/generic/che-packages}"

#[why] unset resolves the newest published catalog rather than failing: a local build wants
#   current definitions, and only CI, where the variable is set, needs an exact one
VERSION="${GRP_VAR_CHE_PACKAGES_REF:-${CHE_PACKAGES_REF:-}}"
if [[ -z "$VERSION" && -z "${CHE_PACKAGES_DIR:-}" ]] {
  print -r -- "GRP_VAR_CHE_PACKAGES_REF unset, resolving latest"
  VERSION="$(curl -fsSL --connect-timeout 30 --retry 5 --retry-delay 5 "${BASE}/latest/version.txt" | tr -d '[:space:]')"
  [[ -n "$VERSION" ]] || { print -ru2 -- "could not resolve latest che-packages version from ${BASE}"; exit 1 }
}

if [[ -z "${CHE_PACKAGES_DIR:-}" && -f "${DEST}/version.txt" && "$(<${DEST}/version.txt)" == "$VERSION" ]] {
  print -r -- "che-packages ${VERSION} already vendored"
  exit 0
}

#[why] CHE_PACKAGES_DIR short-circuits the registry: the local checkout is the source
#   while the catalog's first release is still unpublished, and it makes an offline
#   build (and a catalog change tested before tagging) work without network.
if [[ -n "${CHE_PACKAGES_DIR:-}" ]] {
  [[ -f "${CHE_PACKAGES_DIR}/packages.yml" ]] || { print -ru2 -- "CHE_PACKAGES_DIR has no packages.yml: ${CHE_PACKAGES_DIR}"; exit 1 }
  command rm -rf "$DEST"
  mkdir -p "${DEST}/scripts"
  cp "${CHE_PACKAGES_DIR}/packages.yml" "${DEST}/packages.yml"
  cp -R "${CHE_PACKAGES_DIR}/scripts/." "${DEST}/scripts/"
  print -r -- "${VERSION:-0.0.0}+local" > "${DEST}/version.txt"
  print -r -- "vendored che-packages from ${CHE_PACKAGES_DIR} (local)"
  exit 0
}

NAME="che-packages_${VERSION}.tar.gz"
TMP="$(mktemp -d)"
trap 'command rm -rf "$TMP"' EXIT

print -r -- "fetching ${NAME}"
curl -fsSL --connect-timeout 30 --retry 5 --retry-delay 5 -o "${TMP}/${NAME}" "${BASE}/${VERSION}/${NAME}"
curl -fsSL --connect-timeout 30 --retry 5 --retry-delay 5 -o "${TMP}/checksums.txt" "${BASE}/${VERSION}/checksums.txt"

WANT="$(awk -v n="$NAME" '$2 == n || $2 == "*" n {print $1}' "${TMP}/checksums.txt")"
[[ -n "$WANT" ]] || { print -ru2 -- "checksums.txt has no entry for ${NAME}"; exit 1 }
GOT="$(shasum -a 256 "${TMP}/${NAME}" | cut -d' ' -f1)"
[[ "$WANT" == "$GOT" ]] || { print -ru2 -- "checksum mismatch for ${NAME}: want ${WANT}, got ${GOT}"; exit 1 }

command rm -rf "$DEST"
mkdir -p "$DEST"
tar -xzf "${TMP}/${NAME}" -C "$DEST"
print -r -- "$VERSION" > "${DEST}/version.txt"
print -r -- "vendored che-packages ${VERSION} into ${DEST#${REPO}/}"
##[<] 🤖🤖
