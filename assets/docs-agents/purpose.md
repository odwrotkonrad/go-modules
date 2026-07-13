# Purpose

## What It Is

Go monorepo for all user Go modules: `che` (spec-driven dotfile loader, carrying the shared render engine and doc-rendering CLIs as its `render/` package tree), `get-os-open-files-with`, `get-term-open-files-with`, `lib` (shared config-loading and CLI-main helpers for the get-* CLIs). Each module keeps its own `go.mod` and release stream.

## Why It Exists

Four sibling repos duplicated toolchain, CI, conventions, release flow. One repo carries all: cross-module changes in one MR, render-files folded into che (its only consumer).

## Goals

- One pipeline: per-module test and release jobs fire only for changed dirs.
- Dir-prefixed tags (`<module>/vX.Y.Z`) continue each module's numbering.
- Shared conventions: generated docs, lefthook, Makefile style, one MIT license.
