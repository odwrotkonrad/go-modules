#!/bin/sh
##[>] 🤖
set -eu
export NONINTERACTIVE=1
installer=$(curl -fsSL --connect-timeout 30 --retry 10 --retry-delay 30 --retry-all-errors https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)
[ -n "$installer" ] || exit 1
/bin/bash -c "$installer"
##[<] 🤖
