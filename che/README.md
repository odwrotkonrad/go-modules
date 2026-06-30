# che

Spec-driven dotfile configuration loader.

`che` detects `OS + arch + virt`, resolves a profile from `che.yml`, then loads
only the files, dirs, installs, and services that profile selects.

## Install

```sh
go install gitlab.com/konradodwrot/go/che@latest
```

Requires CGO (the 1Password SDK) — a C compiler must be present at build time.

## Usage

```
che [command]

Available Commands:
  copy             *.host.cp copy op
  detect           print the detected profile and exit
  link             symlink op (configs into system root)
  mk-dirs          create repo-tree dirs + extra-dirs
  prune-links      delete broken symlinks
  render-templates *.host.tpl render op
  run-scripts      run the profile's scripts, optionally filtered by name substring
  services         load/unload/verify the profile's launchd services

Flags:
      --dry-run string[="delta"]   print mutating actions instead of executing them:
                                   delta (changed dests) | all (every dest)
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

## License

MIT — see [LICENSE](LICENSE).
