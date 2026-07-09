che [command]

Available Commands:
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
      --dry-run string[="delta"]   print mutating actions instead of executing them: delta (changed dests) | all (every dest)
      --omit-exec-if               treat every execIf predicate as passing; env: CHE_OMIT_EXEC_IF
      --profile string             run only this profile (autoExec skipped, execIf still enforced); env: CHE_PROFILE
      --skip-plugins               skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS
