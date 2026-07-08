# Purpose

## What It Is

Spec-driven dotfile configuration loader: detects OS+arch+virt, resolves a profile from `che.yml`, loads that profile's files, dirs, installs, services. Renders `*.tpl` templates, each dest path deciding the target (relative: repo, `~/` or absolute: host), resolving op:// (1Password) secret refs at render time.

## Why It Exists

Dotfile loading as shell scripts is fragile, imperative, host-specific. One declarative spec drives every host: the same configs repo loads onto any OS, arch, virt combination through profile selection.

## Goals

- One `che.yml` spec declares files, dirs, installs, services per profile.
- Idempotent host loading: symlink, copy, render, prune, verify.
- Generated repo docs: `*.ontoRepo.tpl` rendering keeps agent files and README fresh.
- Secrets stay in 1Password: op:// refs resolve only at render time.

