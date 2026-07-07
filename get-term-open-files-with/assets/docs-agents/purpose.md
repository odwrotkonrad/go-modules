# Purpose

## What It Is

CLI printing `<ext>=<opener>` zsh suffix-alias lines from `term-open-files-with.yml` (system + user, deep-merged), expanded against cached GitHub-linguist language data. Targets `any|vscode|kitty`.

## Why It Exists

Terminal open-by-extension behavior belongs in config: one YAML declares openers per language, linguist expands language names to extensions. The YAML lives in the configs repo; this CLI turns it into zsh suffix aliases.

## Goals

- Suffix aliases as code, loaded by che.
- Per-terminal targets: `any`, `vscode`, `kitty`.
- Offline after first run: linguist data cached locally.

## How To Use

`go install gitlab.com/konradodwrot/go/cruft/get-term-open-files-with@latest`, run `get-term-open-files-with <target>`, eval the lines in zsh. Development: `make test`, `make build`.

## Future Direction

Tracks the `term-open-files-with.yml` schema and terminal targets as the configs repo grows.
