# Feature: Che Log

<!-- [>] 🤖🤖 -->

Two log kinds: human log for CLI consumption, machine log for otel/prometheus.
The human log is readable prose: headings, indentation, multi-line allowed.
`CHE_LOG_LEVEL` selects error (failures only), warn (adds warnings), info
(default, what happened), debug (adds what is going to happen, what is not
going to happen, why), trace (adds details). Discovery reports each profile,
its working directory, and its ops with delta counts; ops report the mutations
they make or would make.

Scenario: human and machine logs are separate kinds
  Status: tested
  When any che command runs
  Then the CLI prints the human log only
  And the machine log (otel traces/metrics, prometheus) carries the structured events
  And no machine-oriented line format leaks into the human log
  And structure comes from headings and indentation, readability decides the layout

Scenario: CHE_LOG_LEVEL selects the log level
  Status: tested
  When I set `CHE_LOG_LEVEL=<level>` with level error | warn | info | debug | trace
  Then the human log includes that level and every higher-severity level
  And info is the default when unset

Scenario: each level shows itself and every higher-severity level
  Status: tested
  Given severity order error > warn > info > debug > trace
  When I set the log level
  Then error shows only error lines
  And warn shows error and warn lines
  And info shows error, warn and info lines
  And debug shows error, warn, info and debug lines
  And trace shows every line
  And a line below the selected level never prints

Scenario: info logs only what happened
  Status: tested
  Given log level info
  When a che command runs
  Then the human log reports completed facts only
  And no line announces what is going to happen
  And no line explains what is not going to happen

Scenario: debug adds intentions and negatives with reasons
  Status: tested
  Given log level debug
  When a che command runs
  Then the human log additionally reports what is going to happen
  And what is not going to happen, each with its reason

Scenario: trace adds details
  Status: tested
  Given log level trace
  When a che command runs
  Then the human log additionally reports detail-level events
  And details never log at info or debug

Scenario: every level but info carries a level prefix
  Status: tested
  When the human log prints a line
  Then an error line starts with `[error] `
  And a warn line starts with `[warn] `
  And a debug line starts with `[debug] `
  And a trace line starts with `[trace] `
  And an info line carries no level prefix

Scenario: the human log nests profiles and ops as markdown headings
  Status: tested
  When a che command executes a profile's ops at log level info
  Then each profile announces as a `# Run profile <ref>` heading
  And each op it runs announces as a `## <op>` sub-heading beneath it
  And every mutation the op makes is a plain line indented under its op heading
  And an op that changes nothing renders as a heading with a no-changes note, no lines beneath it
  And a line beneath a profile heading carries no repeated profile-name suffix

Scenario: run announces its init and discover stages as headings
  Status: tested
  When `che run` starts at log level info
  Then it announces the init-remote-sources stage as a heading before the remote lines
  And it announces the discover-profiles stage as a heading before the discovered profiles

Scenario: discover logs the spec path
  Status: tested
  When discovery runs at log level info
  Then the human log reports the che spec path in use

Scenario: discover logs each remote once
  Status: tested
  Given the spec references remote sources
  When discovery initializes or updates remotes at log level info
  Then the human log reports one entry per remote
  And each entry states whether the remote was initialized or updated
  And each entry states it landed in cache, with the cache path abbreviated

Scenario: a discovered profile reports its working directory and ops
  Status: tested
  When discovery reports a profile
  Then it announces as a `## Profile <ref>  (profile workdir: <dir>)` heading, one level under the `# discover-profiles` heading
  And it lists the profile's working directory
  And it lists the os-mutating che commands the profile would perform, execution order
  And config-skipped ops (--skip-ops) are excluded
  And every declared op lists, zero-delta included, as `<op>: <changes> (<n> declared)`, at every level that shows discovery
  And at debug each op additionally lists its declared items beneath it, each marked changed or unchanged

Scenario: debug logs profiles that failed discovery
  Status: tested
  Given log level debug
  When discovery rejects a profile
  Then the human log reports the rejected profile with the reason
  And no ops list is needed for a rejected profile

Scenario: dry run reports the true predicted operations
  Status: tested
  When dry-run=delta runs
  Then only operations that would change os state report
  And each predicted mutation renders affirmatively with a `(dry run)` suffix, e.g. `create <dest> (dry run)`, never as a `will not` line
  And dry-run=all additionally reports every settled dest as a no-op: `will not <action> <dest>: <reason>`
  And no-op reasons distinguish already-exists (dirs), already-linked (links), same-content (copies and renders), already-set (perms)
  And a no-op line carries only its reason, not the dry-run mode
  And dry-run=all bypasses the zero-delta profile skip

Scenario: mutation reports created or overwritten
  Status: tested
  When an op mutates a dest
  Then the action is created for a previously absent dest and overwritten for an existing one
  And template renders report under the render-templates op heading

Scenario: render-templates delta comes from a mock-render content compare
  Status: tested
  When discovery counts render-templates deltas
  Then each secret-free template's mock render composes per dest and byte-compares against the dest's current content, differing or absent counts as delta
  And each secret-bearing template's mock-render hash compares against the stored hash of the previous run instead
  And the cache stores the most recent hash only, keyed by dest, written on real renders only

Scenario: a zero-delta op still runs
  Status: tested
  When run reaches an op whose delta is zero
  Then the op still executes (idempotent, sweeps included)
  And its heading notes `(no changes)` at info

Scenario: uninstall groups removals by profile in reverse application order
  Status: tested
  When `che uninstall` runs at log level info
  Then each profile's removals sit under a `profile <ref>` heading
  And the profiles unwind in reverse of the order they were applied
  And each removed dest logs one indented line under its profile heading

Scenario: uninstall skips a non-empty che-created dir
  Status: todo
  When uninstall reverts a dir op whose dest still holds other content
  Then the dir stays and a debug skip line reports `will not remove <dest>: directory not empty`
  And no `removed <dest>` line logs
  And no inverse removal records in the ledger
  And no raw rmdir stderr leaks into the output

Scenario: debug logs the config delta from defaults
  Status: tested
  Given log level debug
  When a che command starts
  Then the human log reports the config options differing from defaults only
  And never the full config

Scenario: config show outputs the config delta by default
  Status: tested
  When I invoke `che config show` or `che config show --delta`
  Then the output lists the config options differing from defaults, with sources
  And --delta is the default mode

Scenario: config show --all outputs the full config
  Status: tested
  When I invoke `che config show --all`
  Then the output lists every config option with its value and source

Scenario: config show prints no delta summary line
  Status: tested
  When I invoke `che config show` in any mode
  Then the output holds only the per-option lines
  And no `config delta ...` summary line precedes them

Scenario: config show --all sorts changed options before defaults
  Status: tested
  When I invoke `che config show --all`
  Then the changed options list first, in config order
  And the remaining options follow, in config order

Scenario: config show labels unset options as unset
  Status: tested
  Given a config option no source sets
  When I invoke `che config show --all`
  Then that option shows its default value labeled `(unset)`, not `(default)`

Scenario: config show labels options explicitly set to their default with the source
  Status: tested
  Given a config option a source sets to its default value
  When I invoke `che config show --all`
  Then that option is labeled with its source, e.g. `(cliFlag)`
  And it sorts with the changed options

<!-- [<] 🤖🤖 -->
