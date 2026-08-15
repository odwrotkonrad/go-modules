# Feature: Build-From-Source Installation Method

<!-- [>] 🤖🤖 -->

`buildFromSource` installs a package by downloading its source tarball and
running the autotools flow inside it: `./configure --prefix=<prefix>`,
`make -j<cpus>`, `make install`. The prefix derives from the configured
binaries install destination (`~/.local/bin` -> `~/.local`), so the default
flow stays in user space and needs no sudo.

Scenario: a user installs a package from its source tarball
  Status: tested
  When a package item declares `buildFromSource` with a `url` (a `{version}` token allowed) and a pinned version
  Then che downloads the tarball, extracts it into a temp dir, and locates its single top-level source dir
  And runs `./configure --prefix=<prefix>` with any `configureArgs` appended, then `make -j<cpus>`, then `make install` inside it
  And the prefix is the parent of the binaries install destination (default `~/.local/bin` -> `~/.local`)
  And the temp dir is removed afterwards

Scenario: a system prefix escalates only the install step
  Status: tested
  When the resolved prefix lies outside `$HOME` (e.g. `/usr/local/bin` -> `/usr/local`)
  Then only `make install` runs under sudo (linux, non-root)
  And configure and make run unprivileged

Scenario: a single declared checksum guards the platform-independent tarball
  Status: tested
  When the item declares `checksum: sha256:<hex>`
  Then the downloaded tarball must match it, a mismatch aborts the install
  And an absent checksum installs with an `unverified` warning
  And `platformEligibility` takes platform names only: source tarballs are one artifact, per-platform checksums are rejected at parse time

Scenario: platform eligibility gates hosts, absence means everywhere
  Status: tested
  When the item declares `platformEligibility` names
  Then the method applies only on those `<os>-<arch>` platforms
  And an empty or absent list applies on every platform where `osInstallers` carries `buildFromSource`

Scenario: an installed matching version skips the build
  Status: tested
  When the package command is present and its version output carries the pin
  Then the install is skipped as already installed
  And a version drift rebuilds from source
  And dry-run only announces `install <pkg> <version> via buildFromSource`

Scenario: build prerequisites are ensured as base packages
  Status: implemented
  When a `buildFromSource` install runs
  Then the `buildFromSource` base-packages group (gcc, make) is ensured first

Scenario: ruby builds from source where no package manager serves it
  Status: implemented
  When ruby is requested and only `buildFromSource` applies
  Then che builds ruby 3.4.10 from the ruby-lang.org tarball, checksum-verified
  And ruby requires libssl-dev, libyaml-dev, zlib1g-dev (apt-only, skipped where inapplicable)
  And ruby-lsp requires ruby itself instead of ruby-dev: a source-built ruby ships its own headers

<!-- [<] 🤖🤖 -->
