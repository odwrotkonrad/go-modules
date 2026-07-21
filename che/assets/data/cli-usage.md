che [command]

Available Commands:
  backup               manage backup archives: create, ls, restore
    create             archive every op dest (links, copies, host renders) into the per-run backup archive and exit
    ls                 list the backup points (run id, backup id, timestamp, size, path), newest first
    restore            restore state from backup archives: --run-id (that run's archives), --backup-id (one archive), --timestamp (point-in-time)
  completion           Generate the autocompletion script for the specified shell
    bash               Generate the autocompletion script for bash
    fish               Generate the autocompletion script for fish
    powershell         Generate the autocompletion script for powershell
    zsh                Generate the autocompletion script for zsh
  config               inspect che's resolved configuration
    show               print the resolved options with their deciding sources (--delta default, --all for every option)
  discover-profiles    log the discovered profiles with their per-op changes and exit
  init-remote-sources  fetch the remote spec sources (clone/pull the cache checkouts) and exit
  make-copies          *.ontoHost.cp copy op
  make-dirs            create repo-tree dirs + extra-dirs
  make-links           symlink op (configs into system root)
  prune-broken-links   delete broken symlinks
  render-templates     render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)
  run                  run every op each profile selects, profile by profile
  run-scripts          run the profile's scripts, optionally filtered by name substring
  uninstall            back out everything che installed (ledger-driven), restoring pre-install backups

Flags:
  -C, --che-working-directory string       change into this directory before resolving the repo; env: CHE_WORKING_DIRECTORY
      --dry-run string[="delta"]           print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest) | true (alias for delta); default: off; env: CHE_DRY_RUN
      --log-level string                   human-log level; values: error (failures only) | warn | info (what happened) | debug (adds intentions and won't-happen with reasons) | trace (adds details); default: info; env: CHE_LOG_LEVEL
      --profile-working-directory string   the load-ops source tree (che level; spec/profile options.profileWorkingDirectory override); default root; env: CHE_PROFILE_WORKING_DIRECTORY
      --profiles strings                   run only these profiles (comma-separated or repeated; autoDiscover skipped, runIf still enforced); env: CHE_PROFILE (comma-separated)
      --skip-ops strings                   skip these ops everywhere (comma-separated or repeated; dropped from the run sequence, direct op subcommands become logged no-ops); values: prune-broken-links | make-dirs | make-links | make-copies | render-templates | run-scripts; env: CHE_SKIP_OPS
      --skip-remote-refs                   skip sourced include.profiles refs, load only the local repo's specs; env: CHE_SKIP_REMOTE_REFS
      --skip-run-if                        treat every runIf predicate as passing; env: CHE_SKIP_RUN_IF
      --validate-spec string               validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC
