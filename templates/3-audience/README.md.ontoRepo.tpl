# go-modules

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Modules

| Module | What |
| --- | --- |
| [che](che/) | Spec-driven dotfile configuration loader, plus the shared render engine under `che/render/`, exposed as `che render {tpl,dirs-tree,makefile-doc,repo-group-index}` |
| [get-os-open-files-with](get-os-open-files-with/) | CLI printing macOS file-handler association lines for duti |
| [get-term-open-files-with](get-term-open-files-with/) | CLI printing zsh suffix-alias lines for terminal file opening |
| [lib](lib/) | Shared library for the CLI modules: `yamlcfg` config loading, `climain` main-shape helpers (no binaries) |

## Install

### che via brew (Homebrew on macOS, Linuxbrew)

```sh
brew tap odwrotkonrad/tap
brew install che
```

Pin a version via its versioned formula:

```sh
brew install che@0.0.67
```

Binary formula for `{darwin,linux}_{amd64,arm64}`: installs `che` from the
prebuilt release archives, updated by each che tag pipeline. The
tap is mirrored to [GitHub](https://github.com/odwrotkonrad/homebrew-tap), which
`brew tap` resolves by default.

### che via apt (Debian, Ubuntu)

```sh
sudo curl -fsSL -o /usr/share/keyrings/che-archive-keyring.asc https://konradodwrot.gitlab.io/go-modules/apt/gpg.key
echo 'deb [signed-by=/usr/share/keyrings/che-archive-keyring.asc] https://konradodwrot.gitlab.io/go-modules/apt stable main' | sudo tee /etc/apt/sources.list.d/che.list
sudo apt update && sudo apt install che
```

Pin a version (the pool keeps every release):

```sh
sudo apt install che=0.0.67
```

Binary package for `linux_{amd64,arm64}`: installs `che`
into `/usr/bin`. The repo is GPG-signed and rebuilt from the package registry
by each che tag pipeline.

### Prebuilt archives

Prebuilt: each [release](https://gitlab.com/konradodwrot/go-modules/-/releases)
`<module>/vX.Y.Z` attaches `<binary>_<version>_{darwin,linux}_{amd64,arm64}.tar.gz`
plus `checksums.txt` (che: also `che.schema.json`).

### From source

```sh
go install gitlab.com/konradodwrot/go-modules/che@latest
go install gitlab.com/konradodwrot/go-modules/get-os-open-files-with@latest
go install gitlab.com/konradodwrot/go-modules/get-term-open-files-with@latest
```

che needs CGO (1Password SDK): a C compiler at build time.

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
tag pipeline goreleaser-builds the module and uploads archives to the generic
package registry, linked as release assets.

## License

MIT, see [LICENSE](LICENSE).
