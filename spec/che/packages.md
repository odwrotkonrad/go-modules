# Feature: Che Packages

<!-- [>] 🤖🤖 -->

`che packages` declaratively installs packages from a packages file
(`$XDG_CONFIG_HOME/packages/packages.yml`): each canonical name lists managers
in preference order (brew/cask/apt/npm/go/gem/binary), the first applicable on
this host wins. Profiles declare `include.installPackages` and the run
sequence installs them before `runScripts`. Four check subcommands report
presence, upgradability, shadowing, and duplicates.

Scenario: a user names a package once, by its CLI name, and every host resolves it
  Status: tested
  When I declare a package in packages.yml under its canonical name (the CLI program name when the package ships one)
  Then `che packages install <name>` installs it on any supported host
  And a bare manager item uses the canonical name, `{manager: name}` overrides it per manager
  And `{brew: {cask: name}}` installs a cask

Scenario: a user steers method selection with preferred installation methods
  Status: tested
  When `packages.preferredInstallationMethods: [methods...]` is set in the user config, a spec's options, a profile's options, `--preferred-methods`, or CHE_PACKAGES_PREFERRED_METHODS
  Then listed managers are tried first (in the given order) within each package's entry
  And unlisted managers follow in entry order, so fallbacks survive
  And an inapplicable preferred manager falls through to the next applicable one
  And the cascade is flag/env > profile > spec > user config
  And an unknown method name is a hard error naming the valid set

Scenario: a user relocates binary installs and knows when the target is off PATH
  Status: tested
  When `packages.binary.installDestinationCandidates` is set (user config, spec options, profile options, or CHE_PACKAGES_BINARY_INSTALL_DESTINATION_CANDIDATES; scalar or list, ~/ and $VARs expand; default ~/.local/bin)
  Then with `checkInPath` (default true) the first candidate found on PATH becomes the install destination
  And when no candidate is on PATH a warning lists them once per run and the first entry is used
  And `checkInPath: false` skips the PATH probe and always uses the first entry

Scenario: a user lists managers in preference order and the first applicable one wins
  Status: tested
  When a package lists several managers
  Then brew/cask apply on macos with brew present, apt on linux with apt-get present
  And npm/go/gem apply where their command is present
  And binary applies when a sha256 exists for this os-arch
  And the first applicable item in entry order installs the package
  And an unknown manager is a hard error
  And a package with no applicable manager is a logged skip, not an error
  And an unknown package name is a hard error naming the packages file

Scenario: a manager installed earlier in the run serves later packages, no second invocation
  Status: tested
  When one install run installs npm via apt and another package needs npm
  Then resolution runs in rounds: the npm-managed package installs in a later round of the same run

Scenario: a profile installs its packages before its scripts
  Status: tested
  When a profile declares `include.installPackages: [names...]`
  Then the run sequence executes install-packages after render-templates and before run-scripts
  And `exclude.installPackages` drops matching names
  And composed profiles' package lists concatenate and dedupe
  And `--skip-ops install-packages` skips the stage

Scenario: an installed package is left alone by default, no surprise updates
  Status: tested
  When a package is installed by its selected manager and no version is specified
  Then install leaves it untouched

Scenario: a version pin converges the host on exactly that version, downgrades included
  Status: tested
  When a version is specified (binary `version:`, npm `name@ver`, apt `name=ver`) and the installed version differs
  Then install reinstalls to match the pin exactly

Scenario: a user refreshes everything with one flag
  Status: tested
  When I invoke `che packages install --update`
  Then unpinned installed packages update via their manager (brew upgrade, apt-get install --only-upgrade, npm update -g)
  And pinned packages still converge on the pin

Scenario: a user fills only the gaps with --if-missing
  Status: tested
  When I invoke `che packages install --if-missing`
  Then a package whose canonical command exists anywhere on PATH is skipped, regardless of manager

Scenario: a bare host installs packages with no packages file at all
  Status: tested
  When no packages file exists at the default path and none is configured
  Then che falls back to its builtin packages.yml (shipped in the binary)
  And an explicitly configured path that is missing stays a hard error

Scenario: a user overrides single entries without forking the packages file
  Status: tested
  When an override file exists (`--packages-override` or `$XDG_CONFIG_HOME/che/packages-override.yml`)
  Then its same-name entries replace the base entries and new names append
  And `--packages-file` / `packages.file` user config relocate the base file

Scenario: a binary entry downloads, verifies, and lands in /usr/local/bin
  Status: tested
  When a binary item applies (sha256 for this os-arch)
  Then url and bin members expand {version} {os} {arch} {arch_x} {arch_g}
  And the download sha256-verifies or the install aborts
  And .tar.* extract the listed members, .zip unzips, bare assets install as-is
  And a pinned version absent from `<pkg> --version` output triggers reinstall

Scenario: a vscode extension is a package like any other
  Status: tested
  When a package named by its extension id lists the `code` manager (`golang.go: [code]`)
  Then it applies where the `code` command is present and installs via `code --install-extension <id>`
  And presence (install skip and check-present) reads `code --list-extensions`, queried once per run, case-insensitive
  And `--update` reruns the install with `--force`
  And installing vscode (cask) and its extensions in one run works: extensions resolve in a later round once `code` exists

Scenario: a vendor installer becomes a declarative script entry
  Status: tested
  When a package lists a `- script:` item (optional `os: darwin|linux` gate) with `run:` inline shell, `path:` a script file, or `url:` a fetched script
  Then it applies on matching hosts and runs via POSIX `/bin/sh -e` when the canonical command is missing
  And a `path:` resolves relative to the packages file, falling back to the POSIX sh scripts shipped inside che (install-brew.sh, install-aws.sh)
  And a `url:` fetches with curl (retrying) and aborts the install when the fetch fails or returns empty
  And a present canonical command skips the script (install-if-missing semantics)
  And an optional `version:` + per os-arch `sha256:` pin declaratively: the pin exports to the script as CHE_PKG_VERSION/CHE_PKG_SHA256 (plus CHE_PKG_NAME/OS/ARCH/ARCH_X/ARCH_G), a `--version` output lacking the pin reinstalls and check-upgradable warns, and a sha256 map gates applicability to hosts with a key (gcloud: apt-repo script on linux, pinned tarball script on macos)
  And dry run announces `install <pkg> via script` without executing

Scenario: a macos .pkg vendor installer is a declarative pkg entry
  Status: tested
  When a package lists a `- pkg:` item (url, optional version + per os-arch sha256)
  Then it applies on macos only: the .pkg downloads, sha256-verifies when pinned, and installs via `sudo installer -pkg <asset> -target /`
  And a present canonical command skips it; a version pin absent from `--version` output reinstalls
  And the linux half of such a package is a sibling `script` item (e.g. aws: pkg on macos, shipped install-aws.sh on linux)

Scenario: an install run ends by proving the commands exist
  Status: tested
  When `che packages install` finishes a real run
  Then check-present runs over the installed set and warns on missing commands
  And no other check runs automatically

Scenario: a user audits presence explicitly with check-present
  Status: tested
  When I invoke `che packages check-present [pkg...]`
  Then each canonical command's PATH presence reports
  And any missing command makes the command fail

Scenario: a user sees what drifted with check-upgradable
  Status: tested
  When I invoke `che packages check-upgradable`
  Then manager-reported outdated packages warn (brew outdated, apt list --upgradable, npm outdated -g)
  And binary entries whose installed --version output lacks the yaml pin warn

Scenario: a user spots PATH shadowing with check-not-shadowed
  Status: tested
  When I invoke `che packages check-not-shadowed`
  Then a package whose manager-expected binary is not the first PATH hit warns `shadowed by <path>`

Scenario: a user spots duplicate installs with check-single-present
  Status: tested
  When I invoke `che packages check-single-present`
  Then a canonical command present in more than one PATH dir warns `multiple-present` listing every location

Scenario: a dry run announces installs without touching the host
  Status: tested
  When I invoke an install under --dry-run
  Then each pending package announces `install <pkg> via <mgr> (dry run)`
  And no manager install command executes

<!-- [<] 🤖🤖 -->
