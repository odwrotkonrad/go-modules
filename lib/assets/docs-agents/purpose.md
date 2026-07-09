# Purpose

## What It Is

Shared library module for the sibling CLI modules: `yamlcfg` (system + user
YAML config loading with deep-merge, `CodedError` exit codes), `climain`
(help/version dispatch, coded-exit epilogue). No binaries, dep on
`gopkg.in/yaml.v3` only.

## Why It Exists

`get-os-open-files-with` and `get-term-open-files-with` carried byte-identical
config loaders and main-shape boilerplate. One module owns the shared code.

## Goals

- One config loader: system + user deep-merge, coded errors, XDG fallback.
- One CLI main shape: `-h/--help`, `-v/--version`, coded exit.
- Minimal dependency surface: yaml.v3 only.
