# render-files

Two doc-rendering CLIs used to generate repo documentation, plus the shared
`checkcmd` helper that backs their `--check` modes.

## Binaries

### `render-makefile-doc`

```
render-makefile-doc <makefile-path>
render-makefile-doc --check <doc-file>
```

Emit `makefile.agents.md` from a Makefile's `[genai-include]` sections (stdout).
`--check` regenerates from `./Makefile` and diffs against `<doc-file>`: exit 0
match, 22 differ (unified diff on stderr).

### `render-dirs-tree`

```
render-dirs-tree
render-dirs-tree --check <file>
```

Print the plain directory tree of the cwd repo's tracked files (stdout): read
tracked paths from the git index, drop each file leaf, nest and sort the
remaining dirs. `--check` regenerates and diffs against `<file>`.

## Install

```sh
go install gitlab.com/konradodwrot/go/render-files/cmd/render-makefile-doc@latest
go install gitlab.com/konradodwrot/go/render-files/cmd/render-dirs-tree@latest
```

`render-makefile-doc` requires CGO (tree-sitter) — a C compiler must be present
at build time.

## Develop

```sh
make test     # go test ./...
make build    # build both binaries into ./dist
make release-snapshot
```

## License

MIT — see [LICENSE](LICENSE).
