# Feature: Signed Apt Repo on Pages

<!-- [>] 🤖🤖 -->

`publish-apt`: every Pages deploy rebuilds the signed apt repo tree at
`https://konradodwrot.gitlab.io/go-modules/apt` from every che `.deb` in the
generic package registry.

Scenario: a user installs the latest che via apt
  Status: implemented
  When a `che/vX.Y.Z` tag pipeline runs `publish-che` then `apt-build-che` and `pages-publish-tag`
  Then `che_X.Y.Z_linux_{amd64,arm64}.deb` uploads to the generic package registry and links as a release asset
  And the Pages site serves `apt/dists/stable` (component `main`) listing it
  And `apt update && apt install che` installs che plus the render CLIs into `/usr/bin`

Scenario: a user pins an exact che version at install time
  Status: implemented
  When releases accumulate
  Then the rebuilt pool carries every version's deb, none removed by later releases
  And `apt install che=X.Y.Z` installs exactly that version, apt-native, no `che@X.Y.Z` package clones

Scenario: a user verifies the repo signature with the published key
  Status: implemented
  When any Pages deploy runs `publish-apt`
  Then the `Release` file is GPG-signed with the key from `$APT_GPG_PRIVATE_KEY`
  And the armored public key serves at `apt/gpg.key` for `signed-by`

Scenario: a docs-only deploy never wipes the apt repo
  Status: implemented
  When a default-branch push touching `che/` deploys Pages
  Then `apt-build-che` regenerates the full tree from the registry first
  And `pages-publish` composes docs + `public-apt` into one site

Scenario: an operator runs the first deploy before any deb exists
  Status: implemented
  When the registry holds no che debs
  Then `publish-apt` publishes only `apt/gpg.key` and exits 0

<!-- [<] 🤖🤖 -->
