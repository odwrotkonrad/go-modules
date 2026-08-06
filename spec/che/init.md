# Feature: Che Init Operation

<!-- [>] 🤖🤖 -->

`init-remote-sources`: prefetch the remote spec sources into the run cache.

Scenario: a user gets every remote source ready up front, nested refs included
  Status: tested
  When I invoke `init-remote-sources` standalone or any command that resolves specs
  Then every remote spec source reachable from the root spec clones or pulls into the cache
  And top-level include.sources and every profile's sourced include.profiles refs are covered, recursively

Scenario: a config author can go offline knowing even conditionally-guarded sources are cached
  Status: tested
  When init reaches a source guarded by runIf conditions
  Then the source still fetches, no condition evaluates
  And discovery later decides what runs

Scenario: a user pays each remote fetch at most once per run, discovery reuses init's checkouts
  Status: tested
  When a che command resolves its specs
  Then init runs before discovery
  And discovery reuses init's checkouts, fetching each source at most once per run

Scenario: a user never runs against a missing source, an unfetchable uncached remote stops the run
  Status: tested
  When a remote source fails to fetch and no cached checkout exists
  Then init errors and the command aborts

Scenario: a user keeps working offline, the cached checkout stands in for an unreachable remote
  Status: tested
  When a remote source fails to update but a cached checkout exists
  Then a warning `fetch failed, using cached checkout <path>` logs and the cached checkout is used

Scenario: an operator traces which profile pulled in which remote dependency
  Status: tested
  When init-remote-sources detects a profile's remote ref
  Then a trace line logs `detected remote ref profile <profile>: <dependency>`

Scenario: an operator reads each remote's fate in one line: cloned, updated, or up to date
  Status: tested
  When init-remote-sources ensures a source
  Then a fresh checkout logs one info line `cloned remote <git-url> into <path>`
  And an updated checkout logs one info line `updated remote <git-url> into <path>`
  And an up-to-date checkout logs one info line `up to date remote <git-url> into <path>`
  And the cache path abbreviates the home prefix to `~`

Scenario: an operator finds every remote checkout under one predictable cache dir
  Status: tested
  When a remote source clones
  Then its checkout lives under `<cache-home>/che/remote-sources/<slug>`

Scenario: a user works fully offline on demand, no remote fetch attempted
  Status: tested
  Given skipRemoteRefs is set
  When init runs
  Then no remote source fetches

<!-- [<] 🤖🤖 -->
