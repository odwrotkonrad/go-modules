# Purpose

## What It Is

CLI printing `<bundle> <uti> <role>` file-handler association lines from `os-open-files-with.yml` (system + user, deep-merged), in config order. Feeds duti on macOS.

## Why It Exists

macOS file associations belong in config, not clicked together in Finder. The YAML lives in the configs repo; this CLI turns it into duti input.

## Goals

- File-handler associations as code, loaded by che.
- Deterministic output: config order, system + user deep-merge.

## How To Use

`go install gitlab.com/konradodwrot/go/cruft/get-os-open-files-with@latest`, run `get-os-open-files-with`, pipe the lines to duti. Development: `make test`, `make build`.

## Future Direction

Tracks the `os-open-files-with.yml` schema as the configs repo grows.
