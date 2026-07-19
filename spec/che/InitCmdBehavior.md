# Feature: Che Init Operation

<!-- [>] 🤖🤖 -->

`init-remote-sources`: prefetch the remote spec sources into the run cache.

Scenario: init fetches every remote spec source
  Status: tested
  When I invoke `init-remote-sources` standalone or any command that resolves specs
  Then every remote spec source reachable from the root spec clones or pulls into the cache
  And top-level include.sources and every profile's sourced include.profiles refs are covered, recursively

Scenario: init fetches without evaluating conditions
  Status: tested
  When init reaches a source guarded by runIf conditions
  Then the source still fetches, no condition evaluates
  And discovery later decides what runs

Scenario: init precedes discover
  Status: tested
  When a che command resolves its specs
  Then init runs before discovery
  And discovery reuses init's checkouts, fetching each source at most once per run

Scenario: a failing fetch without a cache aborts
  Status: tested
  When a remote source fails to fetch and no cached checkout exists
  Then init errors and the command aborts

Scenario: a failing fetch falls back to the cache
  Status: tested
  When a remote source fails to update but a cached checkout exists
  Then an `init-remote-sources(warning)` line logs and the cached checkout is used

Scenario: each detected remote ref logs its dependency
  Status: tested
  When init-remote-sources detects a profile's remote ref
  Then a debug line logs `init-remote-sources(detectedRemoteInSpec): profile=<profile> <dependency>`

Scenario: each remote source logs one line
  Status: tested
  When init-remote-sources ensures a source
  Then a fresh checkout logs one info line `init-remote-sources(cloneRemote): <git-url> -> <os-path>`
  And an updated checkout logs one info line `init-remote-sources(updateRemote): <git-url> -> <os-path>`
  And an up-to-date checkout logs one info line `init-remote-sources(update, skippedDue[RemoteUpToDate]): <git-url> -> <os-path>`

Scenario: remote sources cache under remote-sources
  Status: tested
  When a remote source clones
  Then its checkout lives under `<cache-home>/che/remote-sources/<slug>`

Scenario: skip-remote-refs skips remote fetches
  Status: tested
  Given skipRemoteRefs is set
  When init runs
  Then no remote source fetches

<!-- [<] 🤖🤖 -->
