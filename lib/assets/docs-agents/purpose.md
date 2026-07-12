# Purpose

## What It Is

Shared library module for the sibling CLI modules: `yamlcfg` (system + user
YAML config loading, deep-merge, XDG fallback, `CodedError` exit codes),
`climain` (`-h/--help`, `-v/--version` dispatch, coded-exit epilogue). No
binaries.

## Why It Exists

`get-os-open-files-with` and `get-term-open-files-with` carried byte-identical
config loaders and main-shape boilerplate.

## Goals

- One config loader, one CLI main shape for the get-* CLIs.
- Minimal dependency surface: `gopkg.in/yaml.v3` only.
