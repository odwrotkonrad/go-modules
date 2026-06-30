## `./Makefile`

### Parameters:

`DRY_RUN=delta|all` mode with values
`VERBOSE` plain param no vals
`EXPORTED` bare export, read from env

### Wrappers:

`run-sync-quick`: `run-host-thing -> run-repo-thing` do quick sync

### Onto Host:

`run-host-thing` place a thing on host
`run-host-undocumented`
`run-host-why-only`

### Onto Repo (CI):

`run-repo-thing` outer repo target

#### VM:

`run-vm-nested` nested target
