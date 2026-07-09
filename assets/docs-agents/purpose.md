# Purpose

## What It Is

Go monorepo for all user Go modules: `che` (spec-driven dotfile loader, carrying the shared render engine and doc-rendering CLIs as its `render/` package tree), `get-os-open-files-with`, `get-term-open-files-with`, `lib` (shared config-loading and CLI-main helpers for the get-* CLIs). Each module keeps its own `go.mod` and its own release stream via dir-prefixed tags (`<module>/vX.Y.Z`), continuing the version numbering of the former standalone repos.

## Why It Exists

Four sibling Go repos duplicated the same toolchain, CI shape, conventions scaffolding, and release flow. One repo carries them all: one pipeline with per-module job streams, cross-module changes land in one MR, render-files folded into che (its only consumer).

## Goals

- One repo, one pipeline: per-module test and release jobs fire only for changed dirs.
- Per-module versioning retained: dir-prefixed tags continue each module's old numbering.
- che releases ship the render CLIs (`render-tpl`, `render-dirs-tree`, `render-makefile-doc`, `render-repo-group-index`) alongside `che`.
- Same conventions as sibling repos: generated docs, lefthook, Makefile style, one MIT license.
