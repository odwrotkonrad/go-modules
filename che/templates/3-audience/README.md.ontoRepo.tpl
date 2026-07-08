# che

Spec-driven dotfile configuration loader.

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Install

Prebuilt binaries: every [release](https://gitlab.com/konradodwrot/go/che/-/releases)
attaches `che_<version>_{darwin,linux}_{amd64,arm64}.tar.gz` (plus
`checksums.txt` and `che.schema.json`). Download, extract, put `che` on
`PATH`.

From source:

```sh
go install gitlab.com/konradodwrot/go/che@latest
```

Requires CGO (the 1Password SDK): a C compiler must be present at build time.

## Usage

```
{{ renderMarkdown "assets/data/cli-usage.md" }}
```

## Docs

Rendered: <https://konradodwrot.gitlab.io/go/che> (GitLab Pages, `main`).

- [docs/spec.md](docs/spec.md): `che.yml` reference (profiles, options, include/exclude, per-dest render options).
- [docs/cli.md](docs/cli.md): generated CLI reference (commands, flags, env vars).
- [assets/data/che.schema.json](assets/data/che.schema.json): generated JSON Schema for `che.yml`. Editor validation, first line of `che.yml`:

```yaml
# yaml-language-server: $schema=https://gitlab.com/konradodwrot/go/che/-/raw/main/assets/data/che.schema.json
```

Pin `main` to `v<X.Y.Z>` to match the installed che (each release tag
snapshots the schema; it is also attached as a release asset).

## Develop

```sh
make test     # go test ./...
make build    # build into ./dist
```

Releases are automatic: every push to `main` runs tests, then CI bumps the
patch version from the highest `vX.Y.Z` tag and publishes a GitLab release.
Consumers install with `go install ...@vX.Y.Z` (compiles on the host).

Every MR pipeline publishes a prerelease tag `v<next>-<branch>.<N>` pointing at
the branch commit (`<branch>` sanitized to the SemVer charset). Test branch
code with:

```sh
go install gitlab.com/konradodwrot/go/che@v<next>-<branch>.<N>
```

`-<branch>.<N>` is a SemVer prerelease, so `@latest` still resolves to the
latest stable tag — prereleases are opt-in by explicit tag.

## License

MIT — see [LICENSE](LICENSE).
