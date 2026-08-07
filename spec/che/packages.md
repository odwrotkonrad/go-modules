# Feature: Che Packages

<!-- [>] 🤖🤖 -->

`che packages` declaratively installs packages from a packages file
(`$XDG_CONFIG_HOME/packages/packages.yml`): each canonical name lists managers
in preference order (brew, brew/cask, brew/vscode, apt, npm, go, gem, prebuiltBinariesArchive, script, versionManager), the first applicable on
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

Scenario: a user relocates prebuiltBinariesArchive installs and knows when the target is off PATH
  Status: tested
  When `packages.prebuiltBinariesArchive.installDestinationCandidates` is set (user config, spec options, profile options, or CHE_PACKAGES_PREBUILT_BINARIES_ARCHIVE_INSTALL_DESTINATION_CANDIDATES; scalar or list, ~/ and $VARs expand; default ~/.local/bin)
  Then with `checkPresentOnPath` (default true) the first candidate found on PATH becomes the install destination
  And when no candidate is on PATH a warning lists them once per run and the first entry is used
  And `checkPresentOnPath: false` skips the PATH probe and always uses the first entry

Scenario: a user lists managers in preference order and the first applicable one wins
  Status: tested
  When a package lists several managers
  Then brew/cask apply on macos with brew present, apt on linux with apt-get present
  And npm/go/gem apply where their command is present
  And prebuiltBinariesArchive applies when its platforms list carries this os-arch
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
  When a version is specified (entry-level or spec-level `version:`) and the installed version differs
  Then install reinstalls to match the pin: npm installs `name@<pin>`, apt installs `name=<pin>`, go installs `module@v<pin>`, gem installs `-v <pin>`; unpinnable managers run their update path
  And embedded pins in item names (npm `name@ver`, apt `name=ver`) are parse errors naming the version field

Scenario: an entry-level version guards any package's installed version
  Status: tested
  When an entry sets an exact `version:`
  Then it overrides the item-level pin for the drift check, matched against whole version tokens of the probe output
  And a prebuiltBinariesArchive item whose url or extractBinaries uses `{version}` requires a pinned version: entry or item `version:`, or a requested one; none is a hard error
  And a manager-installed package drifting from the pin runs the manager's update path; check-upgradable warns while drifted
  And entries whose manager ships one recent version stay unpinned by convention; the builtin pins every version-distributing entry (archives, scripts, versionManager, npm/gem/go tools)

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

Scenario: a user's own file ships its own scripts next to it
  Status: tested
  When an entry in a superseding packages file or an override file uses a relative script `path:` (an install method's or `postInstall`'s)
  Then the path resolves against that file's directory, not the base file's or the working directory
  And it never silently falls back to a same-named builtin script

Scenario: top-level archNameConventions and platforms blocks standardize os, arch, and platform references
  Status: tested
  When the packages file opens with `archNameConventions:` (named arch spelling sets, e.g. `uname: {amd64: x86_64, arm64: aarch64}`, `odd: {amd64: x86_64, arm64: arm64}`) and `platforms:` (each supported `<os>-<arch>` id keyed to its supported installation methods)
  Then the platform keys are the supported set: item `platforms:` gates and method lists validate against them, an unknown value is a hard error naming the block
  And on a host whose platform id is a key, only its listed methods are applicable, named explicitly: `brew`, `brew/cask`, `brew/vscode`, apt, npm, go, gem, prebuiltBinariesArchive, script, versionManager; an unlisted host platform falls back to the built-in applicability rules
  And a file without the blocks inherits the builtin's
  And an item's `archConvention: <set>` picks the spelling `{arch}` expands to; every item using `{arch}` must declare it (the builtin ships `go`, `uname`, `odd`)
  And the old `{arch_x}`/`{arch_g}` tokens and per-item `archNames:` maps are parse errors

Scenario: a prebuiltBinariesArchive entry downloads, verifies, and lands on the destination candidate
  Status: tested
  When a prebuiltBinariesArchive item applies (platforms carries this os-arch)
  Then url and extractBinaries members expand {version} {os} {arch} ({arch} spelled per archConvention)
  And the download verifies against the platform's sha256 (a bare platform name skips verification with a warning) or the install aborts
  And .tar.* extract the listed members, .zip unzips, bare assets install as-is
  And a pinned version absent from the version probe output triggers reinstall
  And the probe runs `<canonical> --version`, falling back to `<canonical> version`, unless the entry sets `versionCommand:` (odd tools, e.g. kubectl -> `kubectl version --client`)

Scenario: a vscode extension is a package like any other
  Status: tested
  When a package named by its extension id lists the `vscode` method via the brew object form (`golang.go: [{brew: {vscode: golang.go}}]`; bare `code`/`vscode` and `{code: id}`/`{vscode: id}` are illegal)
  Then it applies where the `code` command is present and installs via `code --install-extension <id>`
  And presence (install skip and check-present) reads `code --list-extensions`, queried once per run, case-insensitive
  And `--update` reruns the install with `--force`
  And installing vscode (cask) and its extensions in one run works: extensions resolve in a later round once `code` exists

Scenario: a vendor installer becomes a declarative script entry
  Status: tested
  When a package lists a `- script:` item (optional `os: darwin|linux` gate) with `run:` inline shell, `path:` a script file, or `remoteUrl:` a fetched script
  Then it applies on matching hosts and runs via POSIX `/bin/sh -e` when the canonical command is missing
  And a `path:` resolves relative to the packages file
  And a `remoteUrl:` fetches with curl (retrying) and aborts the install when the fetch fails or returns empty
  And a present canonical command skips the script (install-if-missing semantics)
  And an optional `version:` + `platforms:` (platform names, each optionally `platform: sha256`) pin declaratively: the pin exports to the script as CHE_PKG_VERSION/CHE_PKG_SHA256 (plus CHE_PKG_NAME/OS/ARCH and CHE_PKG_ARCH_<SET> per archNameConventions set), a `--version` output lacking the pin reinstalls and check-upgradable warns, and a platforms list gates applicability to listed hosts (gcloud: apt-repo on linux, pinned tarball script on macos)
  And dry run announces `install <pkg> via script` without executing

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
  And prebuiltBinariesArchive entries whose installed --version output lacks the yaml pin warn

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
