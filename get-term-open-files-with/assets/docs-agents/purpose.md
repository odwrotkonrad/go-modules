# Purpose

## What It Is

CLI printing `<ext>=<opener>` zsh suffix-alias lines from `term-open-files-with.yml` (system + user, deep-merged), expanded against cached GitHub-linguist language data. Targets `any|vscode|kitty`.

## Why It Exists

Terminal open-by-extension behavior belongs in config: one YAML declares openers per language. The YAML lives in the configs repo.

## Goals

- Suffix aliases as code, loaded by che.
- Offline after first run.

