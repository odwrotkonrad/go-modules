# go-modules

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Modules

| Module | What |
| --- | --- |
| [che](che/) | Spec-driven dotfile configuration loader, plus the shared render engine and doc-rendering CLIs (`render-tpl`, `render-dirs-tree`, `render-makefile-doc`, `render-repo-group-index`) under `che/render/` |
| [get-os-open-files-with](get-os-open-files-with/) | CLI printing macOS file-handler association lines for duti |
| [get-term-open-files-with](get-term-open-files-with/) | CLI printing zsh suffix-alias lines for terminal file opening |
| [lib](lib/) | Shared library for the CLI modules: `yamlcfg` config loading, `climain` main-shape helpers (no binaries) |

## Install

Prebuilt binaries: each [release](https://gitlab.com/konradodwrot/go-modules/-/releases)
`<module>/vX.Y.Z` attaches `<binary>_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`
plus `checksums.txt` (che releases: also the render CLI archives and
`che.schema.json`).

From source:

```sh
go install gitlab.com/konradodwrot/go-modules/che@latest
go install gitlab.com/konradodwrot/go-modules/che/render/cmd/render-tpl@latest
go install gitlab.com/konradodwrot/go-modules/get-os-open-files-with@latest
go install gitlab.com/konradodwrot/go-modules/get-term-open-files-with@latest
```

che and the render CLIs need CGO (1Password SDK): a C compiler at build time.

## Docs

Rendered che docs: <https://konradodwrot.gitlab.io/go-modules> (GitLab Pages, `main`).

- [che/docs/spec.md](che/docs/spec.md): `che.yml` reference.
- [che/docs/cli.md](che/docs/cli.md): generated CLI reference.
- [che/assets/data/che.schema.json](che/assets/data/che.schema.json): generated JSON Schema, editor wiring in [spec.md](che/docs/spec.md#editor-validation).

## Develop

```sh
make test     # go test ./... in every module
make build    # build every module into <module>/dist
```

Releases are per module, automatic: a push to `main` touching `<module>/` runs
its tests, then CI bumps the patch from the highest `<module>/vX.Y.Z` tag. The
tag pipeline goreleaser-builds the module and uploads the archives to the
generic package registry, linked as release assets.

## License

MIT, see [LICENSE](LICENSE).
