# Feature: Che Human and Machine Log Split

<!-- [>] 🤖🤖 -->

Two log kinds: human log for CLI consumption, machine log for otel/prometheus.
The human log drops the `scope(label, label)` line form: readable prose,
headings, indentation, multi-line allowed. Log level replaces the debug flag:
`CHE_LOG_LEVEL` selects error (failures only), warn (adds warnings), info
(default, what happened), debug (adds what is going to happen, what is not
going to happen, why), trace (adds details). `CHE_DEBUG` is retired.

Scenario: human and machine logs are separate kinds
  Status: tested
  When any che command runs
  Then the CLI prints the human log only
  And the machine log (otel traces/metrics, prometheus) carries the structured events
  And no machine-oriented line format leaks into the human log

Scenario: the human log drops the label form
  Status: tested
  When any che command prints a human log line
  Then no line uses the `scope(label, label): value` form
  And structure comes from headings and indentation, not parenthesized labels
  And a logical unit may span multiple lines

Scenario: one self-contained line per log is no longer required
  Status: tested
  When the human log reports an event
  Then it need not fit one self-contained line
  And readability for the user decides the layout: headings, indentation, multi-line

Scenario: CHE_LOG_LEVEL selects the log level
  Status: tested
  When I set `CHE_LOG_LEVEL=<level>` with level error | warn | info | debug | trace
  Then the human log includes that level and every level above it
  And info is the default when unset

Scenario: CHE_DEBUG is retired
  Status: tested
  When I set `CHE_DEBUG`
  Then che ignores it, `CHE_LOG_LEVEL` is the only level switch

Scenario: error logs failures only
  Status: tested
  Given log level error
  When a che command runs
  Then the human log reports failures only
  And nothing else logs

Scenario: warn adds warnings
  Status: tested
  Given log level warn
  When a che command runs
  Then the human log reports failures and warnings only

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

Scenario: discover logs profiles with ops and deltas
  Status: tested
  When discovery matches profiles at log level info
  Then the human log reports each discovered profile
  And each profile lists its working directory
  And each profile lists its ops
  And each op lists the changes it is going to make, delta only

Scenario: debug logs the config delta from defaults
  Status: tested
  Given log level debug
  When a che command starts
  Then the human log reports the config options differing from defaults only
  And never the full config

Scenario: debug logs profiles that failed discovery
  Status: tested
  Given log level debug
  When discovery rejects a profile
  Then the human log reports the rejected profile with the reason
  And no ops list is needed for a rejected profile

Scenario: each executed op announces itself, its mutations nested under it
  Status: tested
  When a che command executes a profile's ops at log level info
  Then the human log announces each op it runs, indented under the profile
  And every mutation the op makes indents one level deeper, under that op
  And an op that changes nothing announces itself with a no-changes note, no lines beneath it

Scenario: config show outputs the config delta by default
  Status: tested
  When I invoke `che config show` or `che config show --delta`
  Then the output lists the config options differing from defaults, with sources
  And --delta is the default mode

Scenario: config show --all outputs the full config
  Status: tested
  When I invoke `che config show --all`
  Then the output lists every config option with its value and source

<!-- [<] 🤖🤖 -->
