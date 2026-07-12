# Purpose

## What It Is

CLI printing `<bundle> <uti> <role>` file-handler association lines from `os-open-files-with.yml` (system + user, deep-merged), in config order. Feeds duti on macOS.

## Why It Exists

macOS file associations belong in config, not clicked together in Finder. The YAML lives in the configs repo.

## Goals

- Associations as code, loaded by che.
- Deterministic output.

