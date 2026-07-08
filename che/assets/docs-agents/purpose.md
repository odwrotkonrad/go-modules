# Purpose

## What It Is

Spec-driven dotfile configuration loader: detects OS+arch+virt, resolves a profile from `che.yml`, loads that profile's files, dirs, installs, services. Renders `*.tpl` templates, each dest path deciding the target (relative: repo, `~/` or absolute: host), resolving op:// (1Password) secret refs at render time. The `render/` package tree carries the shared gomplate render engine plus doc-rendering CLIs: `render-tpl` (gomplate built-ins, op:// secrets, `remoteFile` cross-repo inclusion, frontmatter, markdown transforms), `render-makefile-doc` (`[genai-include]` Makefile docs), `render-dirs-tree` (tracked-file directory trees), `render-repo-group-index` (subgroup repo indexes), `checkcmd` (`--check` drift helper).

## Why It Exists

Dotfile loading as shell scripts is fragile, imperative, host-specific. One declarative spec drives every host: the same configs repo loads onto any OS, arch, virt combination through profile selection. che and repo docs generation share one render implementation, so the render engine lives here, in its only consumer.

## Goals

- One `che.yml` spec declares files, dirs, installs, services per profile.
- Idempotent host loading: symlink, copy, render, prune, verify.
- Generated repo docs: `*.ontoRepo.tpl` rendering keeps agent files and README fresh.
- Secrets stay in 1Password: op:// refs resolve only at render time.
- One render engine for host and repo templates across all repos, drift detection via `--check`.
- che releases ship the render CLIs alongside the `che` binary.
