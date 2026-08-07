#!/bin/sh
##[>] 🤖
#[where] https://docs.cloud.google.com/sdk/docs/downloads-versioned-archives
set -eu
[ "${CHE_PKG_OS}" = darwin ] || { echo "darwin only, linux installs via apt" >&2; exit 1; }
platform=darwin-arm
[ "${CHE_PKG_ARCH}" = arm64 ] || platform=darwin-x86_64
archive="google-cloud-cli-${CHE_PKG_VERSION}-${platform}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors -O \
  "https://storage.googleapis.com/cloud-sdk-release/${archive}"
echo "${CHE_PKG_SHA256}  ${archive}" | shasum -a 256 -c -
SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO=sudo
$SUDO rm -rf /usr/local/google-cloud-sdk
$SUDO tar -xzf "$archive" -C /usr/local
$SUDO /usr/local/google-cloud-sdk/install.sh --quiet --usage-reporting false --path-update false
for bin in gcloud gsutil bq; do
  $SUDO ln -fs "/usr/local/google-cloud-sdk/bin/${bin}" "/usr/local/bin/${bin}"
done
##[<] 🤖
