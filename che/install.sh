#!/bin/sh
##[>] 🤖🤖
set -eu

err() {
  printf 'che install: %s\n' "$1" >&2
  exit 1
}

for tool in uname tar mktemp; do
  command -v "$tool" >/dev/null 2>&1 || err "missing required tool: $tool"
done

#[why] no --retry-all-errors: a missing asset (404) is not transient, and retrying it stalls the
#   caller for retry * retry-delay before the inevitable failure. curl's default retry set already
#   covers the transient cases (timeouts, 408, 429, 5xx)
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 -o "$1" "$2"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$1" "$2"; }
else
  err "missing required tool: curl or wget"
fi

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) os="$(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) arch="$(uname -m)" ;;
esac
case "${os}_${arch}" in
  darwin_arm64 | linux_amd64 | linux_arm64) ;;
  *) err "unsupported platform: ${os}/${arch}" ;;
esac

version="${CHE_VERSION:-latest}"
url="https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che/${version}/che_${version}_${os}_${arch}.tar.gz"

use_sudo=""
usable() {
  case ":${PATH}:" in
    *":$1:"*) ;;
    *) return 1 ;;
  esac
  use_sudo=""
  if [ -d "$1" ] && [ -w "$1" ]; then return 0; fi
  if [ ! -e "$1" ] && mkdir -p "$1" 2>/dev/null; then return 0; fi
  if [ "$1" = /usr/local/bin ] && command -v sudo >/dev/null 2>&1; then
    if sudo -n true 2>/dev/null || sudo true; then
      use_sudo=sudo
      sudo mkdir -p "$1"
      return 0
    fi
  fi
  return 1
}

if [ -n "${CHE_INSTALL_DIR:-}" ]; then
  set -- "$CHE_INSTALL_DIR"
else
  set -- "${HOME}/.local/bin" "${HOME}/bin"
fi
dir=""
for candidate in "$@"; do
  if usable "$candidate"; then
    dir="$candidate"
    break
  fi
done
[ -n "$dir" ] || err "no writable install dir on PATH (candidates: $*) PATH=${PATH}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fetch "${tmp}/che.tar.gz" "$url"
tar -xzf "${tmp}/che.tar.gz" -C "$tmp" che

if [ -n "$use_sudo" ]; then
  sudo install -m 0755 "${tmp}/che" "${dir}/che"
else
  install -m 0755 "${tmp}/che" "${dir}/che"
fi

"${dir}/che" --version
printf 'installed che %s into %s\n' "$version" "$dir"
##[<] 🤖🤖
