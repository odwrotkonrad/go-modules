# che

Spec-driven dotfile configuration loader.

{{ renderMarkdown "assets/docs-agents/purpose.md" "normalize-headings" }}

## Install

```sh
go install gitlab.com/konradodwrot/go/che@latest
```

Requires CGO (the 1Password SDK) — a C compiler must be present at build time.

## Usage

```
che [command]

Available Commands:
  copy             *.ontoHost.cp copy op
  detect           print the detected profile and exit
  link             symlink op (configs into system root)
  mk-dirs          create repo-tree dirs + extra-dirs
  prune-links      delete broken symlinks
  render-templates *.tpl render op (dest path decides host vs repo)
  run-scripts      run the profile's scripts, optionally filtered by name substring
  services         load/unload/verify the profile's launchd services

Flags:
      --dry-run string[="delta"]   print mutating actions instead of executing them:
                                   delta (changed dests) | all (every dest)
      --profile string             run only this profile (onlyIf skipped, mixinOnly allowed)
  -v, --version                    version for che
```

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
