# Purpose

## What It Is

Shared gomplate render engine plus doc-rendering CLIs: `render-tpl` (gomplate built-ins, op:// 1Password secrets, `remoteFile` cross-repo inclusion, frontmatter, markdown transforms), `render-makefile-doc` (`[genai-include]` Makefile docs), `render-dirs-tree` (tracked-file directory trees), `checkcmd` (`--check` drift helper). Library consumed by che.

## Why It Exists

che and repo docs generation share one render implementation. This repo is its home: template funcs, composition (autogen headers, `@`-includes, env mergeUpsert), native doc generators, standalone CLIs.

## Goals

- One render engine for host and repo templates across all repos.
- Generated docs stay fresh: Makefile docs, directory trees, agent files, README.
- Drift detection via `--check` modes.

## How To Use

Import the `render` package (as che does), or install the CLIs with `go install gitlab.com/konradodwrot/go/render-files/cmd/<name>@latest` (`render-makefile-doc` needs CGO for tree-sitter). Development: `make test`, `make build`.

## Future Direction

Grows with the conventions rollout: new template funcs (like `remoteFile` for inlining convention docs) land here first, che consumes them by version bump.
