#!/bin/sh
##[>] 🤖🤖
set -eu

err() {
  printf 'che install: %s\n' "$1" >&2
  exit 1
}

usage() {
  cat <<'EOF'
usage: che-install.sh [options]

Installs the che binary for this platform (darwin/arm64, linux/amd64, linux/arm64)
from the GitLab generic package registry. Every option has an environment variable
twin, the option wins when both are set.

  option                      env                                      meaning
  --version <ver>             INSTALL_CHE_VERSION=<ver>                version to install (default: latest)
  --install-dir <dir>         INSTALL_CHE_DIR=<dir>                    target dir, must be on PATH (default: ~/.local/bin, then ~/bin)
  --skip-if-present           INSTALL_CHE_SKIP_IF_PRESENT=1            exit 0 when any che is on PATH, no network
  --skip-if-present-is-newer  INSTALL_CHE_SKIP_IF_PRESENT_IS_NEWER=1   exit 0 when the che on PATH is newer than the wanted version
  -h, --help                                                           print this help
Exit 0 always prints one outcome line:
  installed che <ver> into <dir>
  skip (present)
  skip (<installed> installed, wanted <ver>)

Examples:
  curl -fsSL https://konradodwrot.gitlab.io/go-modules/che-install.sh | sh
  curl -fsSL https://konradodwrot.gitlab.io/go-modules/che-install.sh | INSTALL_CHE_VERSION=0.0.99 sh -s -- --skip-if-present-is-newer
EOF
}

version_opt=""
install_dir_opt=""
skip_if_present="${INSTALL_CHE_SKIP_IF_PRESENT:-0}"
skip_if_present_is_newer="${INSTALL_CHE_SKIP_IF_PRESENT_IS_NEWER:-0}"
while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || err "--version needs a value"; version_opt="$2"; shift ;;
    --version=*) version_opt="${1#--version=}" ;;
    --install-dir) [ $# -ge 2 ] || err "--install-dir needs a value"; install_dir_opt="$2"; shift ;;
    --install-dir=*) install_dir_opt="${1#--install-dir=}" ;;
    --skip-if-present) skip_if_present=1 ;;
    --skip-if-present-is-newer) skip_if_present_is_newer=1 ;;
    -h | --help) usage; exit 0 ;;
    *) err "unknown argument: $1 (see --help)" ;;
  esac
  shift
done

ref="${version_opt:-${INSTALL_CHE_VERSION:-latest}}"
install_dir="${install_dir_opt:-${INSTALL_CHE_DIR:-}}"

for tool in uname tar mktemp install sort; do
  command -v "$tool" >/dev/null 2>&1 || err "missing required tool: $tool"
done

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

installed="$(che --version 2>/dev/null | awk '{print $3}' || true)"

if [ "$skip_if_present" = 1 ] && [ -n "$installed" ]; then
  printf 'skip (present)\n'
  exit 0
fi

pkg="https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che/${ref}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

if [ "$ref" = latest ]; then
  fetch "${tmp}/version.txt" "${pkg}/che_latest_version.txt" || err "cannot resolve latest version"
  wanted="$(tr -d '[:space:]' < "${tmp}/version.txt")"
  [ -n "$wanted" ] || err "cannot resolve latest version"
  archive="che_latest_${os}_${arch}.tar.gz"
else
  wanted="$ref"
  archive="che_${wanted}_${os}_${arch}.tar.gz"
fi

is_installed_newer() {
  [ -n "$installed" ] || return 1
  [ "$installed" != dev ] || return 1
  [ "$(printf '%s\n%s\n' "$wanted" "$installed" | sort -V | tail -n 1)" = "$installed" ]
}

if [ "$wanted" = "$installed" ]; then
  printf 'skip (%s installed, wanted %s)\n' "$installed" "$wanted"
  exit 0
fi
if [ "$skip_if_present_is_newer" = 1 ] && is_installed_newer; then
  printf 'skip (%s installed, wanted %s)\n' "$installed" "$wanted"
  exit 0
fi

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

if [ -n "$install_dir" ]; then
  set -- "$install_dir"
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

fetch "${tmp}/che.tar.gz" "${pkg}/${archive}"
tar -xzf "${tmp}/che.tar.gz" -C "$tmp" che

if [ -n "$use_sudo" ]; then
  sudo install -m 0755 "${tmp}/che" "${dir}/che"
else
  install -m 0755 "${tmp}/che" "${dir}/che"
fi

printf 'installed che %s into %s\n' "$wanted" "$dir"
##[<] 🤖🤖
