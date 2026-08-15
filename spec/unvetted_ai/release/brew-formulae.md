# Feature: Versioned Brew Formulae

<!-- [>] 🤖🤖 -->

`publish-brew`: each che tag pipeline renders and commits the brew formulae to the homebrew tap repo.

Scenario: a user installs the latest che with a plain tap and install
  Status: implemented
  When a `che/vX.Y.Z` tag pipeline runs `publish-brew`
  Then `Formula/che.rb` (class `Che`) commits to the tap at that version
  And `brew tap odwrotkonrad/tap && brew install che` installs it from the GitHub mirror

Scenario: a user pins an exact che version at install time
  Status: implemented
  When a `che/vX.Y.Z` tag pipeline runs `publish-brew`
  Then `Formula/che@X.Y.Z.rb` also commits to the tap
  And its class name follows Homebrew's formula naming, `che@0.0.67` -> `CheAT0067`
  And `brew install che@X.Y.Z` installs exactly that version

Scenario: a user picks any previously released version, old formulae never disappear
  Status: implemented
  When releases accumulate
  Then each release adds its own `Formula/che@X.Y.Z.rb`, none is removed or rewritten by later releases
  And every versioned formula stays installable, its urls pinned to that version's package registry path

Scenario: an operator re-runs a tag pipeline without breaking the tap
  Status: implemented
  When `publish-brew` commits a formula file that already exists or is missing
  Then the commit falls back between update and create, the re-run succeeds with the same content

Scenario: an operator dry-renders both formulae locally, no tap write
  Status: implemented
  Given `RENDER_ONLY=1`
  When `publish-brew` runs
  Then both formula files render to disk and the script exits before any tap commit

<!-- [<] 🤖🤖 -->
