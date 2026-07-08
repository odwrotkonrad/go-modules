# go-modules

Go monorepo: `che`, `get-os-open-files-with`, `get-term-open-files-with`.

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Modules

| Module | What |
| --- | --- |
| [che](che/) | Spec-driven dotfile configuration loader, plus the shared render engine and doc-rendering CLIs (`render-tpl`, `render-dirs-tree`, `render-makefile-doc`, `render-repo-group-index`) under `che/render/` |
| [get-os-open-files-with](get-os-open-files-with/) | CLI printing macOS file-handler association lines for duti |
| [get-term-open-files-with](get-term-open-files-with/) | CLI printing zsh suffix-alias lines for terminal file opening |

## Install

Prebuilt binaries: each [release](https://gitlab.com/konradodwrot/go-modules/-/releases)
`<module>/vX.Y.Z` attaches `<binary>_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`
(plus `checksums.txt`; che releases also attach the render CLI archives and
`che.schema.json`). Download, extract, put the binary on `PATH`.

From source:

```sh
go install gitlab.com/konradodwrot/go-modules/che@latest
go install gitlab.com/konradodwrot/go-modules/che/render/cmd/render-tpl@latest
go install gitlab.com/konradodwrot/go-modules/get-os-open-files-with@latest
go install gitlab.com/konradodwrot/go-modules/get-term-open-files-with@latest
```

che (and the render CLIs) require CGO (the 1Password SDK): a C compiler must be
present at build time.

## Docs

Rendered che docs: <https://konradodwrot.gitlab.io/go-modules> (GitLab Pages, `main`).

- [che/docs/spec.md](che/docs/spec.md): `che.yml` reference (profiles, options, include/exclude, per-dest render options).
- [che/docs/cli.md](che/docs/cli.md): generated CLI reference (commands, flags, env vars).
- [che/assets/data/che.schema.json](che/assets/data/che.schema.json): generated JSON Schema for `che.yml`. Editor validation, first line of `che.yml`:

```yaml
# yaml-language-server: $schema=https://gitlab.com/konradodwrot/go-modules/-/raw/main/che/assets/data/che.schema.json
```

Pin `main` to `<module>/v<X.Y.Z>` to match the installed che (each release tag
snapshots the schema; it is also attached as a release asset).

## Develop

```sh
make test     # go test ./... in every module
make build    # build every module into <module>/dist
```

Releases are per module and automatic: every push to `main` touching
`<module>/` runs that module's tests, then CI bumps the patch version from the
highest `<module>/vX.Y.Z` tag and publishes a GitLab release. The tag pipeline
builds the module with goreleaser (snapshot mode, version injected from the
tag) and uploads the archives to the generic package registry
(`packages/generic/<module>/<version>/`), linking them as release assets.
Consumers install with `go install ...@vX.Y.Z` (compiles on the host).

## License

MIT — see [LICENSE](LICENSE).
