# Purpose

## What It Is

Shared gomplate render engine plus doc-rendering CLIs: `render-tpl` (gomplate built-ins, op:// 1Password secrets, `remoteFile` cross-repo inclusion, frontmatter, markdown transforms), `render-makefile-doc` (`[genai-include]` Makefile docs), `render-dirs-tree` (tracked-file directory trees), `checkcmd` (`--check` drift helper). Library consumed by che.

## Why It Exists

che and repo docs generation share one render implementation. This repo is its home: template funcs, composition (autogen headers, `@`-includes, env mergeUpsert), native doc generators, standalone CLIs.

## Goals

- One render engine for host and repo templates across all repos.
- Generated docs stay fresh: Makefile docs, directory trees, agent files, README.
- Drift detection via `--check` modes.

