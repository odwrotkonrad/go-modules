#!/bin/sh
##[>] 🤖
#[where] https://docs.cloud.google.com/sdk/docs/install-sdk#deb
set -eu
SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO=sudo
export DEBIAN_FRONTEND=noninteractive
$SUDO apt-get update
$SUDO apt-get install --yes --no-install-recommends apt-transport-https ca-certificates gnupg curl
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors \
  https://packages.cloud.google.com/apt/doc/apt-key.gpg | $SUDO gpg --dearmor --yes -o /usr/share/keyrings/cloud.google.gpg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
  | $SUDO tee /etc/apt/sources.list.d/google-cloud-sdk.list >/dev/null
$SUDO apt-get update
$SUDO apt-get install --yes --no-install-recommends google-cloud-cli
##[<] 🤖
