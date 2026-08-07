#!/bin/sh
##[>] 🤖
set -eu
pyenv install -s "$CHE_PKG_VERSION"
pyenv global "$CHE_PKG_VERSION"
##[<] 🤖
