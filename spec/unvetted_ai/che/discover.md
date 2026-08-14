# Feature: Che Discover Operation

<!-- [>] 🤖🤖 -->

`discover-profiles`: expose the resolved runtime spec `che run` would execute.

An os-mutating che command mutates os state: `run`, `backup`, `prune-broken-links`,
`make-dirs`, `make-links`, `make-copies`, `render-templates`, `run-scripts`,
`uninstall` (ledger-driven, no discovery).

Scenario: a user never mutates os state from a stale plan, every mutating command works from fresh discovery
  Status: tested
  When I invoke an os-mutating che command other than `uninstall` (ledger-driven, no profiles)
  Then discover runs first
  And its result determines the profile execution sequence

Scenario: a user pays discovery cost once per run, not per wrapped command
  Status: tested
  When I invoke `run`
  Then discover runs once, not per wrapped command

Scenario: a config author previews exactly what a run would do without touching the host
  Status: tested
  When I invoke `discover-profiles` standalone
  Then the log lists discovered profiles
  And each profile lists the os-mutating operations it would perform

Scenario: an operator scans discovered profiles as headings, each profile's ops indented beneath
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke an os-mutating che command
  Then the log reports each discovered profile as a `### Profile <ref>` heading, its ops indented beneath

Scenario: an operator reads the runtime spec once at the top, never repeated through the log
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke `run`
  Then the runtime spec logs once, at execution log start

Scenario: a user running one op directly still sees the full plan before it executes
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke an os-mutating che command directly, not as a command wrapped by `run`
  Then discover runs before the command
  And the log lists discovered profiles
  And each profile lists the os-mutating operations it would perform

Scenario: a user gets consistent results across a run's wrapped commands, all working from one discovery
  Status: tested
  When `run` executes its wrapped commands
  Then no wrapped command runs discover again
  And each uses the discovery `run` ran once at start

Scenario: a user sees the discovery plan with no log-level tuning
  Status: tested
  Given `CHE_LOG_LEVEL` is unset
  When I invoke `discover-profiles` standalone or an os-mutating che command
  Then the log lists discovered profiles

Scenario: a config author opting out of auto-discovery gets a clear ask for --profiles, never a surprise full run
  Status: tested
  Given options.autoDiscover is false (che config, default true)
  When I invoke a che command without --profiles
  Then discovery is disabled and the command errors, asking for --profiles
  And forced profiles and sourced refs still run

<!-- [<] 🤖🤖 -->
