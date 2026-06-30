# get-term-open-files-with

```
get-term-open-files-with <any|vscode|kitty>
```

Print `<ext>=<opener>` zsh suffix-alias lines for the terminal from
`term-open-files-with.yml` (system + user, deep-merged) expanded against
GitHub-linguist language data (cached, fetched on first run).

## Install

```sh
go install gitlab.com/konradodwrot/go/cruft/get-term-open-files-with@latest
```

## Develop

```sh
make test
make build
make release-snapshot
```

## License

MIT — see [LICENSE](LICENSE).
