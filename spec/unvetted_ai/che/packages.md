# Feature: Che Packages

<!-- [>] 🤖🤖 -->

`che packages` declaratively installs packages from a packages file
(`$XDG_CONFIG_HOME/packages/packages.yml`): each canonical name lists managers
in preference order (brew, brew/cask, apt, npm, go, gem, binariesRemoteArchive, script, pyenv, nvm, nix), the first applicable on
this host wins. Profiles declare `include.installPackages` and the run
sequence installs them before `runScripts`. Four check subcommands report
presence, upgradability, shadowing, and duplicates.

Scenario: a package named once, by its CLI name, resolves on every host
  Status: tested
  When I declare a package in packages.yml under its canonical name (the CLI program name when the package ships one)
  Then `che packages install <name>` installs it on any supported host
  And a bare installer item uses the canonical name, `installerVocabulary.packageName` overrides it per installer
  And casks are installer keys: `brew/cask` items install casks via brew; the `vscode` installer key is gone, extensions live in `toolPackages.vscode`

Scenario: a user installs a package via a supported installation method
  Status: tested
  When a user wants to install a package via an installation method: brew, brew/cask, apt, npm, go, gem, binariesRemoteArchive, script, pyenv, nvm, nix
  Then they have the ability to do so on any platform where the method is eligible (osInstallers: every platform carries binariesRemoteArchive, script, npm, go, gem, pyenv, nvm, nix. linux-debian adds apt, darwin adds brew, brew/cask)
  And a method outside the set is a hard error naming the valid set

Scenario: a user steers method selection with preferred installation methods
  Status: tested
  When `packages.preferredInstallationMethods: [methods...]` is set in the user config, a spec's options, a profile's options, `--preferred-methods`, or CHE_PACKAGES_PREFERRED_METHODS
  Then listed managers are tried first (in the given order) within each package's entry
  And unlisted managers follow in entry order, so fallbacks survive
  And an inapplicable preferred manager falls through to the next applicable one
  And the cascade is flag/env > profile > spec > user config
  And an unknown method name is a hard error naming the valid set

Scenario: a user relocates binariesRemoteArchive installs and knows when the target is off PATH
  Status: tested
  When `packages.binariesRemoteArchive.installDestinationCandidates` is set (user config, spec options, profile options, or CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_INSTALL_DESTINATION_CANDIDATES; scalar or list, ~/ and $VARs expand; default ~/.local/bin)
  Then with `checkPresentOnPath` (default true) the first candidate found on PATH becomes the install destination
  And when no candidate is on PATH a warning lists them once per run and the first entry is used
  And `checkPresentOnPath: false` skips the PATH probe and always uses the first entry

Scenario: managers list in preference order, the first applicable wins
  Status: tested
  When a package lists several managers
  Then brew/cask apply on macos with brew present, apt on linux with apt-get present
  And go/gem apply where their command is present; npm applies even without the command when no other item is strictly applicable, bootstrapping node first
  And binariesRemoteArchive applies when its installerVocabulary.platformEligibility list carries this os-arch
  And the first applicable item in entry order installs the package
  And an unknown manager is a hard error
  And a requested package with no applicable installation method is a hard error (`--missing-method-warn` downgrades it to a warning)
  And a dependency-pulled package with no applicable installation method is a logged skip
  And an unknown package name is a hard error naming the packages file

Scenario: a manager installed earlier in the run serves later packages, no second invocation
  Status: tested
  When one install run installs npm via apt and another package needs npm
  Then resolution runs in rounds: the npm-managed package installs in a later round of the same run

Scenario: an npm package on a bare host bootstraps node first
  Status: implemented
  When a package's npm item is selected on a host without npm (no item strictly applicable)
  Then the npm `basePackages` group installs first: `node`, pulling `nvm` via requires, nvm's script pulling its own script base group (curl, git, tar, unzip): nested base groups are ensured, each once per run
  And the npm invocation resolves through the nvm default node's bin dir when npm is not on PATH
  And `--only-methods npm` does not filter the bootstrap (base installs bypass only-methods)

Scenario: the install log tells spec-requested packages from dependencies
  Status: implemented
  When an install run installs or skips a package pulled in by `requires` or a manager's `basePackages` group
  Then its log lines label it `<requirer> dependency <pkg>` (`will not install curl dependency git: already installed via apt`)
  And spec-requested packages keep their bare name, even when another entry also requires them
  And a reinstall line shows the currently installed version when known (`reinstall curl: 8.5.0-2 -> 8.14.1`), keeping `-> <pin>` alone otherwise
  And an unpinned apt installed line reports the installed version (`installed apt-transport-https 2.6.1 via apt`); pinned installs keep the pin
  And an updated line reports the manager's post-update version when known

Scenario: a profile installs its packages before its scripts
  Status: tested
  When a profile declares `include.installPackages: [names...]`
  Then the run sequence executes install-packages after render-templates and before run-scripts
  And `exclude.installPackages` drops matching names
  And composed profiles' package lists concatenate and dedupe
  And `--skip-ops install-packages` skips the stage

Scenario: an installed package is left alone by default
  Status: tested
  When a package is installed by its selected manager and no version is specified
  Then install leaves it untouched

Scenario: a version pin converges the host on exactly that version, downgrades included
  Status: tested
  When a version is specified (entry-level or item-level `version:`) and the installed version differs
  Then install reinstalls to match the pin: npm installs `name@<pin>`, apt installs `name=<pin>`, go installs `module@v<pin>`, gem installs `-v <pin>`; unpinnable managers run their update path
  And embedded pins in item names (npm `name@ver`, apt `name=ver`, brew `name@ver`) are parse errors naming the version field
  And install and dry-run messages label the package with its pinned version (`installed aws 2.36.18 via binariesRemoteArchive`)

Scenario: an absent version means rolling, a stated version is a promise checked per installer
  Status: tested
  When an entry or item states no `version:`
  Then the installer tracks its manager's current release: no pin, no drift check
  And an entry states a version only when every installer honors it; otherwise each pinning item declares its own (`{npm: {packageName: <n>, version: <v>}}`, archive and script items via their `version:` field)
  And an item-level version beats the entry version; `latest` means no pin (track head)
  And a version an installer cannot enforce fires a warning and installs the current release (brew/cask: casks are rolling)
  And the old `__rolling__` sentinel is a parse error: omit version instead

Scenario: a pinned brew item derives its versioned formula, the name stays bare
  Status: tested
  When a brew item carries a pin (entry or item `version:`)
  Then brew installs the versioned formula `<name>@<pin>` (`version: "5"` -> `brew install ffmpeg@5`): the suffix is derived, never written
  And a literal version suffix in `installerVocabulary.packageName` (`ffmpeg@5`) is a parse error: names stay bare, the pin is appended automatically
  And an unpinned brew item installs the bare formula
  And a formula line that auto-updates within its suffix needs no finer version: the suffix is the pin

Scenario: an apt package whose version string differs from the binary version maps it once
  Status: tested
  When an apt item's debian package version is decorated beyond the binary version (epoch, revision: `1:2.39.5-0+deb12u3`) or its stream diverges from the entry version
  Then the item declares `installerVocabulary.versionMap: {"<binary-version>": "<package-version>"}`: apt installs `name=<package-version>` (downgrades allowed), the drift check compares dpkg's version to the package string, check-upgradable probes for the binary version
  And the map holds exactly one pair; the item names exactly one deb via `installerVocabulary.packageName` (multi-deb bundles are split into own entries)

Scenario: apt repositories are declared once as registries, entries reference them by url
  Status: tested
  When third-party or non-default apt repositories are needed (docker, google-cloud, bookworm-backports)
  Then the file lists them under `installerRegistries.apt` and an apt item references one by scheme-less url: `fromRegistry: <host/path>[::<suites>[::<components>]]`, narrowing with suites/components when one url serves several registries; an ambiguous reference is a hard error naming the narrowing syntax
  And installing the item configures the registry (keyring + deb822 file named by the registry slug `<url>-<suites>-<components>`, shared by every entry referencing it) even when the package is already installed, so a pruned repo file heals
  And a registry's `verificationKey` accepts a url (key downloaded into /etc/apt/keyrings) or an absolute path (keyring already on the host, nothing fetched)
  And a registry with explicit `suites:` installs with `-t <suites>` so exact-version dependencies (curl -> libcurl4) resolve from that suite too
  And a reference to an undeclared registry, or a registry missing url or verificationKey, is a hard error

Scenario: a nix item installs from nixpkgs via nix profile
  Status: implemented
  When a package lists a `- nix` item (bare, or `{nix: {packageName: <attr>, versionMap: ..., aliasBinary: ..., verify: ...}}`)
  Then it is eligible on linux and darwin, last in the default preference order, so it never preempts another applicable manager
  And on a host without nix the `nix` basePackages group bootstraps nix first via the Determinate Systems installer script (daemon install on hosts, `--init none` in containers; `/nix/receipt.json` marks it installed), and che falls back to `/nix/var/nix/profiles/default/bin/nix` or `~/.nix-profile/bin/nix` while nix is off PATH
  And flake sources are declared once under `installerRegistries.nix` (`name`, `url` flake base, optional `ref` branch); an item picks one via `fromRegistry: <name>`, defaulting to `nixpkgs`; the builtin ships `nixpkgs` (nixpkgs-unstable) and a version-named stable registry (`nixpkgs-26.05`: the nixos-26.05 branch); a reference to an undeclared registry is a hard error
  And an unpinned item installs `nix profile install <url>/<ref>#<attr>` (attr defaults to the entry key, `packageName` overrides), install lines name the registry (`installed bat via nix (nixpkgs-26.05)`), `--update` runs `nix profile upgrade <attr>`
  And presence and installed-version read `nix profile list` (store-path parse, defensive across nix output formats; an unparseable version falls back to the `--version` probe)
  And a packageName carrying `#`, `@`, or `=` is a parse error: names stay bare nixpkgs attributes
  And `platformEligibility`, `extractBinaries`, and `archScheme` on a nix item are parse errors

Scenario: a nix pin is a registry-repo revision
  Status: implemented
  When a nix item pins via `versionMap: {"<binary-version>": "<revision>"}` (exactly one pair, revision of the item's registry repo)
  Then install resolves the ref `<registry-url>/<revision>#<attr>` and the drift check compares the profile's store-path version to the binary version
  And a drifted install reinstalls: `nix profile remove <attr>`, then the pinned ref installs
  And a requested or entry-pinned version with no revision in the versionMap is a hard error naming versionMap
  And every builtin nix item pins: version and revision from the registry's channel head (nixpkgs-unstable, nixos-26.05), one rev shared per registry

Scenario: brew taps are declared once as registries, entries reference them by tap name
  Status: tested
  When a formula or cask lives in a third-party tap
  Then the file lists the tap under `installerRegistries.brew` (`cirruslabs/cli`) and the brew item references it directly: `fromRegistry: cirruslabs/cli` (packageName defaults to the package key)
  And installing taps the repo, trusts the tap-qualified name, then installs it
  And a tap-qualified item name (`packageName: user/tap/name`) is a parse error naming the registries block
  And a reference to a tap absent from the block is a hard error

Scenario: an entry-level version guards any package's installed version
  Status: tested
  When an entry sets an exact `version:`
  Then it overrides the item-level pin for the drift check, matched against whole version tokens of the probe output
  And a binariesRemoteArchive item whose url or installerVocabulary.extractBinaries uses `{version}` requires a pinned version: entry or item `version:`, or a requested one; none is a hard error
  And a manager-installed package drifting from the pin runs the manager's update path; check-upgradable warns while drifted
  And entries whose manager ships one recent version stay unpinned by convention; the builtin pins every version-distributing entry (archives, scripts, pyenv/nvm, npm/gem/go tools)

Scenario: --update refreshes everything
  Status: tested
  When I invoke `che packages install --update`
  Then unpinned installed packages update via their manager (brew upgrade, apt-get install --only-upgrade, npm update -g)
  And pinned packages still converge on the pin

Scenario: --if-missing fills only the gaps
  Status: tested
  When I invoke `che packages install --if-missing`
  Then a package whose canonical command exists anywhere on PATH is skipped, regardless of manager

Scenario: a bare host installs packages with no packages file at all
  Status: tested
  When no packages file exists at the default path and none is configured
  Then che falls back to its builtin packages.yml (shipped in the binary)
  And an explicitly configured path that is missing stays a hard error

Scenario: an override file replaces single entries, no fork of the packages file
  Status: tested
  When an override file exists (`--packages-override` or `$XDG_CONFIG_HOME/che/packages-override.yml`)
  Then its same-name entries replace the base entries and new names append
  And `--packages-file` / `packages.file` user config relocate the base file

Scenario: a user's own file ships its own scripts next to it
  Status: tested
  When an entry in a superseding packages file or an override file uses a relative script `path:` (an install method's or `postInstall`'s)
  Then the path resolves against that file's directory, not the base file's or the working directory
  And it never silently falls back to a same-named builtin script

Scenario: top-level archLabelSchemes and osInstallers blocks standardize arch labels and installer eligibility
  Status: tested
  When the packages file opens with `archLabelSchemes:` (named arch spelling sets, e.g. `uname: {amd64: x86_64, arm64: aarch64}`, `odd: {amd64: x86_64, arm64: arm64}`) and `osInstallers:` (keys from bare os to distro- and arch-qualified: `darwin`, `linux`, `linux-debian`)
  Then the most specific key matching the host wins (`<os>-<distro>-<arch>` > `<os>-<distro>` > `<os>-<arch>` > `<os>`) and only its listed installers are applicable; a host matching no key falls back to the built-in applicability rules
  And apt is eligible only under `linux-debian` (the linux distro read from /etc/os-release ID): apt is not common to every distro, plain `linux` hosts skip apt items
  And item `installerVocabulary.platformEligibility:` ids are `<os>-<arch>`, validated against the block's os prefixes and archLabelSchemes' arches; an unknown value is a hard error naming both
  And on a host whose platform id is a key, only its listed methods are applicable, named explicitly: `brew`, `brew/cask`, apt, npm, go, gem, binariesRemoteArchive, script, pyenv, nvm, nix; an unlisted host platform falls back to the built-in applicability rules
  And a key extends another with a yaml anchor/alias, no installer repetition (`linux: &common [script, npm]`, `linux-debian: [apt, *common]`): alias lists flatten
  And a file without the blocks inherits the builtin's
  And an item's `installerVocabulary.archLabelScheme: <set>` picks the spelling `{arch}` expands to; every item using `{arch}` must declare it (the builtin ships `go`, `uname`, `odd`)
  And the old `{arch_x}`/`{arch_g}` tokens and per-item `archNames:` maps are parse errors

Scenario: a binariesRemoteArchive entry downloads, verifies, and lands on the destination candidate
  Status: tested
  When a binariesRemoteArchive item applies (installerVocabulary.platformEligibility carries this os-arch)
  Then url and installerVocabulary.extractBinaries members expand {version} {os} {arch} ({arch} spelled per archLabelScheme)
  And the download verifies against the platform's checksum, declared algorithm-prefixed (`sha256:<hex>`, an unprefixed value is a parse error); a bare platform name skips verification with a warning, a mismatch aborts the install
  And .tar.* extract the listed members, .zip unzips, bare assets install as-is
  And a pinned version absent from the version probe output triggers reinstall
  And the probe runs `<canonical> --version`, falling back to `<canonical> version`, unless the entry sets `versionCommand:` (odd tools, e.g. kubectl -> `kubectl version --client`)

Scenario: a vscode extension lives in toolPackages, not the manager pipeline
  Status: tested
  When a packages file names extensions under `toolPackages.vscode` (extension id to version pin, null/empty value meaning rolling)
  Then `che packages install --kind=vscode <id>` installs it via `code --install-extension <id>[@pin]`
  And a `vscode` installer key inside `packages:` is a parse error pointing at `toolPackages.vscode`

Scenario: a vendor installer becomes a declarative script entry
  Status: tested
  When a package lists a `- script:` item (optional `os: darwin|linux` gate) with `run:` inline shell, `path:` a script file, or `url:` a fetched script
  Then it applies on matching hosts and runs via POSIX `/bin/sh -e` when the canonical command is missing
  And a `path:` resolves relative to the packages file
  And a `url:` fetches with curl (retrying) and aborts the install when the fetch fails or returns empty
  And a present canonical command skips the script (install-if-missing semantics)
  And an optional `version:` + `installerVocabulary.platformEligibility:` (platform names, each optionally `platform: sha256:<hex>`) pin declaratively: the pin exports to the script as CHE_PKG_VERSION/CHE_PKG_SHA256 (the bare hex, algorithm prefix stripped; plus CHE_PKG_NAME/OS/ARCH and CHE_PKG_ARCH_<SET> per archLabelSchemes set), a `--version` output lacking the pin reinstalls and check-upgradable warns, and a platformEligibility list in the vocabulary gates applicability to listed hosts (gcloud: apt-repo on linux, pinned tarball script on macos)
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
  And binariesRemoteArchive entries whose installed --version output lacks the yaml pin warn

Scenario: a user spots PATH shadowing with check-not-shadowed
  Status: tested
  When I invoke `che packages check-not-shadowed`
  Then a package whose manager-expected binary is not the first PATH hit warns `shadowed by <path>`

Scenario: a user spots duplicate installs with check-single-present
  Status: tested
  When I invoke `che packages check-single-present`
  Then a canonical command present in more than one PATH dir warns `multiple-present` listing every location

Scenario: packages.yml has a json schema with its structure
  Status: tested
  When `make render-docs` runs
  Then it generates the packages file json schema at `assets/data/packages.schema.json` from the Go source, alongside che.schema.json, published with the docs
  And a packages file opens with a `# yaml-language-server: $schema=<url-or-path>` modeline pointing at it, so yaml-aware editors validate and complete entries in place
  And the builtin packages.yml carries that modeline referencing the published schema url

Scenario: a dry run announces installs without touching the host
  Status: tested
  When I invoke an install under --dry-run
  Then each pending package announces `install <pkg> via <mgr> (dry run)`
  And no manager install command executes

Scenario: a user restricts installs to chosen methods with --only-methods
  Status: tested
  When I invoke `che packages install --only-methods <mgr>[,<mgr>...]` (env `CHE_PACKAGES_ONLY_METHODS`)
  Then only entry items using a listed manager are considered, nothing falls through to another installer
  And a package with no listed manager applicable skips with `no applicable installation method`
  And an unknown manager name fails validation

Scenario: a package manager's index is refreshed before its first install of the run
  Status: tested
  Given a run about to install via a manager with a repo index (apt, brew, brew/cask)
  When the manager's first install of the run starts
  Then the manager's repo-update command runs first (apt: `apt-get update`, brew: `brew update`)
  And it runs at most once per run, re-armed when a new apt registry is configured
  And managers querying live registries (npm, gem, go) run no update command

Scenario: a user caches binariesRemoteArchive downloads across runs
  Status: tested
  When I invoke an install with `--download-cache-dir <dir>` (env `CHE_PACKAGES_DOWNLOAD_CACHE_DIR`)
  Then assets download to `<dir>/<sha256(url)>-<basename>` and later installs reuse the cached file without curl
  And a checksum mismatch deletes the cached file before failing
  And an empty value keeps the current per-install temp-dir behavior

<!-- [<] 🤖🤖 -->
