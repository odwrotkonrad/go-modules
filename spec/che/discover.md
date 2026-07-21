# Feature: Che Discover Operation

<!-- [>] 🤖🤖 -->

`discover-profiles`: expose the resolved runtime spec `che run` would execute.

An os-mutating che command mutates os state: `run`, `backup`, `prune-broken-links`,
`make-dirs`, `make-links`, `make-copies`, `render-templates`, `run-scripts`,
`uninstall` (ledger-driven, no discovery).

Scenario: discover precedes every os-mutating che command
  Status: tested
  When I invoke an os-mutating che command other than `uninstall` (ledger-driven, no profiles)
  Then discover runs first
  And its result determines the profile execution sequence

Scenario: run runs discover once
  Status: tested
  When I invoke `run`
  Then discover runs once, not per wrapped command

Scenario: standalone discover lists profiles with their operations
  Status: tested
  When I invoke `discover-profiles` standalone
  Then the log lists discovered profiles
  And each profile lists the os-mutating operations it would perform

Scenario: discovery logs profiles
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke an os-mutating che command
  Then the log reports each discovered profile as a `### Profile <ref>` heading, its ops indented beneath

Scenario: run logs the runtime spec once
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke `run`
  Then the runtime spec logs once, at execution log start

Scenario: a single command logs its operations
  Status: tested
  Given a profile whose runtime spec contains os-mutating operations
  When I invoke an os-mutating che command directly, not as a command wrapped by `run`
  Then discover runs before the command
  And the log lists discovered profiles
  And each profile lists the os-mutating operations it would perform

Scenario: commands wrapped by `run` reuse its discovery
  Status: tested
  When `run` executes its wrapped commands
  Then no wrapped command runs discover again
  And each uses the discovery `run` ran once at start

Scenario: discovery logs at the default log level
  Status: tested
  Given `CHE_LOG_LEVEL` is unset
  When I invoke `discover-profiles` standalone or an os-mutating che command
  Then the log lists discovered profiles

Scenario: auto-discovery can be disabled
  Status: tested
  Given options.autoDiscover is false (che config, default true)
  When I invoke a che command without --profiles
  Then discovery is disabled and the command errors, asking for --profiles
  And forced profiles and sourced refs still run

<!-- [<] 🤖🤖 -->
