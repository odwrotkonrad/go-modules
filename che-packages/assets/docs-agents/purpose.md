# Purpose

## What It Is

Data-only module holding the builtin package database: `packages.yml` (installer definitions, tool packages, registries, verify metadata) plus the `scripts/` it references. One embed-only Go file exports both for che; a tag pipeline publishes the same files as a tarball to the generic package registry for `che packages update`.

## Why It Exists

The definitions lived embedded in che source, so every definition edit required a che release. A separate module gives definitions their own release stream: che pins a version at build time, hosts fetch newer definitions without a che upgrade.

## Goals

- Definitions release independently: `che-packages/vX.Y.Z` tags, tarball + `latest/version.txt` in the generic package registry.
- Near-zero Go: one embed file, no dependencies.
- Per-package e2e coverage keys off this module's edits.
