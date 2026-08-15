#!/bin/sh
##[>] 🤖🤖
set -eu
app=/Applications/kitty.app/Contents
bin="${XDG_BIN_HOME:-$HOME/.local/bin}"
mkdir -p "$bin"
ln -fs "$app/MacOS/kitten" "$bin/kitten"
ln -fs "$app/MacOS/kitty" "$bin/kitty"
man="${XDG_DATA_HOME:-$HOME/.local/share}/man"
cd "$app/Resources/man"
find . -type d -exec mkdir -p "$man/{}" ';'
find . -type f -exec cp -f "{}" "$man/{}" ';'
##[<] 🤖🤖
