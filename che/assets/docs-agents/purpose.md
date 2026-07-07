# Purpose

## What It Is

Spec-driven dotfile configuration loader: detects OS+arch+virt, resolves a profile from `che.yml`, loads that profile's files, dirs, installs, services. Renders `*.host.tpl` onto the host and `*.repo.tpl` onto the repo, resolving op:// (1Password) secret refs at render time.

## Why It Exists

Dotfile loading as shell scripts is fragile, imperative, host-specific. One declarative spec drives every host: the same configs repo loads onto any OS, arch, virt combination through profile selection.

## Goals

- One `che.yml` spec declares files, dirs, installs, services per profile.
- Idempotent host loading: symlink, copy, render, prune, verify.
- Generated repo docs: `*.repo.tpl` rendering keeps agent files and README fresh.
- Secrets stay in 1Password: op:// refs resolve only at render time.

## How To Use

`go install gitlab.com/konradodwrot/go/che@latest` (CGO, needs a C compiler). `che detect` prints the resolved profile; `che link|copy|render-templates|mk-dirs|run-scripts|services` load it. `--dry-run` prints mutating actions instead of executing them. Development: `make test`, `make build`.

## Future Direction

Render engine for the conventions rollout: repos inline convention docs via `remoteFile` in their templates. Releases stay automatic: patch bump per main push, prerelease tags per MR.
