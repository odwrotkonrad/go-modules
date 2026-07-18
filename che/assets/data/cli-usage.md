che [command]

Available Commands:
  all               run every op each profile selects, profile by profile
  completion        Generate the autocompletion script for the specified shell
    bash            Generate the autocompletion script for bash
    fish            Generate the autocompletion script for fish
    powershell      Generate the autocompletion script for powershell
    zsh             Generate the autocompletion script for zsh
  discover          print the prepared profiles (one per line) and exit
  make-copies       *.ontoHost.cp copy op
  make-dirs         create repo-tree dirs + extra-dirs
  make-links        symlink op (configs into system root)
  prune-links       delete broken symlinks
  render-templates  render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)
  run-scripts       run the profile's scripts, optionally filtered by name substring
  uninstall         back out everything che installed (ledger-driven), restoring pre-install backups

Flags:
      --debug                      print debug-level lines (source announce, clone/pull attempts); env: CHE_DEBUG
  -C, --directory string           change into this directory before resolving the repo; env: CHE_DIR
      --dry-run string[="delta"]   print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest) | true (alias for all); default: off; env: CHE_DRY_RUN
      --profiles strings           run only these profiles (comma-separated or repeated; autoDiscover skipped, execIf still enforced); env: CHE_PROFILE (comma-separated)
      --skip-exec-if               treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF
      --skip-ops strings           skip these ops everywhere (comma-separated or repeated; dropped from the all sequence, direct op subcommands become logged no-ops); values: prune-links | make-dirs | make-links | make-copies | render-templates | run-scripts; env: CHE_SKIP_OPS
      --skip-remote-refs           skip sourced include.profiles refs, load only the local repo's specs; env: CHE_SKIP_REMOTE_REFS
      --validate-spec string       validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC
      --working-directory string   the load-ops source tree (che level; spec/profile options.workingDirectory override); default root; env: CHE_WORKING_DIRECTORY
