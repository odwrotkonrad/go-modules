#!/bin/sh
##[>] 🤖🤖
set -eu

local_file="${1:-che/che-install.sh}"
published_url="https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che/latest/che-install.sh"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$tmp" "$published_url" 2>/dev/null || exit 0
else
  wget -q -O "$tmp" "$published_url" || exit 0
fi

local_sum="$(sha256sum "$local_file" | cut -d' ' -f1)"
published_sum="$(sha256sum "$tmp" | cut -d' ' -f1)"
[ "$local_sum" != "$published_sum" ]
##[<] 🤖🤖
