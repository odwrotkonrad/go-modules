che [command]

Available Commands:
  all               run every op the profile selects, in order
  completion        Generate the autocompletion script for the specified shell
    bash            Generate the autocompletion script for bash
    fish            Generate the autocompletion script for fish
    powershell      Generate the autocompletion script for powershell
    zsh             Generate the autocompletion script for zsh
  copy              *.ontoHost.cp copy op
  detect            print the eligible profiles (comma-joined) and exit
  link              symlink op (configs into system root)
  mk-dirs           create repo-tree dirs + extra-dirs
  prune-links       delete broken symlinks
  render-templates  render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)
  run-scripts       run the profile's scripts, optionally filtered by name substring
  services          load/unload/verify the profile's launchd services
    bootin          load each service (bootstrap from plist)
    bootout         unload each service (bootout if loaded, wait until gone)
    ensure          settle then verify each long-running service has a live pid

Flags:
      --debug                      print debug-level lines (plugin announce, clone/pull attempts); env: CHE_DEBUG
  -C, --directory string           change into this directory before resolving the repo; env: CHE_DIR
      --dry-run string[="delta"]   print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest); default: off; env: CHE_DRY_RUN
      --profile string             run only this profile (autoExec skipped, execIf still enforced); env: CHE_PROFILE
      --skip-exec-if               treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF
      --skip-plugins               skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS
      --validate-schema string     validate each loaded che.yml against its JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SCHEMA
