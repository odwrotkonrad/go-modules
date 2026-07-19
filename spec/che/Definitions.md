# Definitions

<!-- [>] 🤖🤖 -->

## che command

A che CLI subcommand, e.g. make-dirs, all, discover, completions.

## os-mutating operation

A single os state mutation: one symlink created, file copied or rendered, dir
created, mode or owner applied, dest removed, script executed.

## os-mutating che command

A che command that performs os-mutating operations.

- os-mutating: `prune-broken-links`, `make-dirs`, `make-links`, `make-copies`,
  `render-templates`, `run-scripts`, `uninstall`, `run` (wraps the above)
- not: `discover`, `completions`
- dry run performs no mutations, classification unchanged

<!-- [<] 🤖🤖 -->
