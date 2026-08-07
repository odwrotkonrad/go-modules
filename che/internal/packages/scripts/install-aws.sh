#!/bin/sh
##[>] 🤖
set -eu
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
case "$(uname -m)" in
  arm64|aarch64) arch=aarch64 ;;
  x86_64|amd64) arch=x86_64 ;;
  *) echo "unknown arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors "https://awscli.amazonaws.com/awscli-exe-linux-${arch}.zip" -o awscliv2.zip
unzip -q awscliv2.zip
SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO=sudo
$SUDO ./aws/install --update
##[<] 🤖
