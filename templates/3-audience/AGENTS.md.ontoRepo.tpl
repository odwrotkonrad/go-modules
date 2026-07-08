# go-modules

Go monorepo: che (+ render engine and CLIs), get-os-open-files-with, get-term-open-files-with

@assets/docs-agents/purpose.md

{{ remoteFile "gitlab.com/konradodwrot/conventions//conventions/conventions.md" }}

# Modules

Each module has its own `go.mod` (`gitlab.com/konradodwrot/go-modules/<module>`) and its own release stream: dir-prefixed tags `<module>/vX.Y.Z`, bumped by CI on default-branch pushes touching the module dir. `go.work` at the root ties the modules together for local dev.

## che

@che/assets/docs-agents/purpose.md

## get-os-open-files-with

@get-os-open-files-with/assets/docs-agents/purpose.md

## get-term-open-files-with

@get-term-open-files-with/assets/docs-agents/purpose.md

@assets/data/makefile.agents.md

## Directory Tree

@assets/data/repo-structure.md
