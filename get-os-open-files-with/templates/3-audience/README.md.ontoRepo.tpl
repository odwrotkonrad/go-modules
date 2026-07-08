# get-os-open-files-with

```
get-os-open-files-with
```

Print `<bundle> <uti> <role>` file-handler association lines from
`os-open-files-with.yml` (system + user, deep-merged), in config order.

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Install

```sh
go install gitlab.com/konradodwrot/go/cruft/get-os-open-files-with@latest
```

## Develop

```sh
make test
make build
```

Releases are automatic: every push to `main` runs tests, then CI bumps the
patch version from the highest `vX.Y.Z` tag and publishes a GitLab release.

## License

MIT — see [LICENSE](LICENSE).
