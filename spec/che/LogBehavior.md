# Feature: Che Discover Log on STDOUT

<!-- [>] 🤖🤖 -->

Statuses: todo | implemented | tested (implemented, tests in place).

Line format, one per discovered profile, profileWorkingDirectory first, ops in
execution order:

```
discover-profiles(match): <profile>: profileWorkingDirectory: <dir>, ops: [<op>(delta=<n>,all=<n>),<op>(delta=<n>,all=<n>)]
```

```
discover-profiles(match): cli/linux: profileWorkingDirectory: /home/ko/configs/root, ops: [prune-broken-links(delta=0,all=12),make-links(delta=2,all=20),render-templates(delta=1,all=10)]
```

Scenario: a discovered profile logs one STDOUT line
  Status: tested
  When discovery logs a profile
  Then one line lists the profile and the os-mutating che commands it would perform
  And profileWorkingDirectory is the first key, the ops array second
  And config-skipped ops (--skip-ops) are excluded from the ops array
  And the commands appear in execution order
  And each command carries delta=<n> operations that would change os state now, first
  And each command carries all=<n> os-mutating operations declared

Scenario: discovered lines log at info level
  Status: tested
  When I invoke `discover-profiles` standalone or discovery precedes an os-mutating che command
  Then the discovered lines log at info level, regardless of debug mode

Scenario: run announces each op it runs
  Status: tested
  When `run` starts one of its wrapped ops, discover included
  Then an info line `run(runOp): <op>` precedes the op's lines
  And a zero-delta op announces `run(runOp, skippedDue[NoDelta]): <op>` at info instead, still executing
  And config-skipped ops announce `run(skippedDueConfigSkipOps): <op>`, deselected ones `run(skippedDueNoDef): <op>`, at info

Scenario: an unchanged render dest logs at debug only
  Status: tested
  When render-templates produces byte-identical content for a dest
  Then no info line logs and the dest is not rewritten
  And a debug line logs `render-templates(overwrite, skippedDue[SameContent]): <dest>`

Scenario: the config log opens every run
  Status: tested
  When a che subcommand starts, before any stage
  Then an info line `config(show, noDefaults): [key=value(source),...]` lists the options a non-default layer decided
  And the source is cliFlag | env | config-file | specFile
  And with debug enabled a debug line `config(show): [...]` lists every option with its value and source

Scenario: the init stage clones remote sources
  Status: tested
  When a che command resolves its specs
  Then remote spec source detectedRemoteInSpec/cloneRemote/updateRemote/skipDueRemoteUpToDate/skip lines log under the `init-remote-sources` scope

Scenario: run announces its init and discover stages
  Status: implemented
  When `run` starts
  Then an info line `run(runOp): init-remote-sources` precedes the init stage's lines
  And an info line `run(runOp): discover-profiles` precedes the discovery log, conditions included

Scenario: discovery lists the candidate profiles at debug
  Status: tested
  When discovery prepares a spec with debug enabled
  Then two debug lines log `discover-profiles(showCandidates, all): [<profile>[<assembled-from>,...],...]` and `discover-profiles(showCandidates, autoDiscoverable): [<profiles>]`
  And a profile assembled from other profiles lists them in brackets

Scenario: condition evaluation logs as part of discovery
  Status: tested
  When discovery evaluates a profile's runIf condition
  Then a debug line reports it as `discover-profiles(evaluating[<profile>], pass|noPass): <condition>`
  And it logs after the discover stage announce, not within init

Scenario: debug lines carry a debug prefix
  Status: tested
  When a debug-level line prints with debug mode enabled
  Then the line starts with the word `debug`, e.g. `debug discover-profiles(evaluating[p], pass): ...`
  And info lines carry no prefix

Scenario: dry run reports the true predicted operations
  Status: tested
  When dry-run=delta runs
  Then only operations that would change os state log, as their real action lines
  And dry-run=all additionally logs every settled dest's state:
  And skip lines shaped `<op>(<action>, skippedDue[<reasons>])`: reasons `AlreadyExists` (dirs), `AlreadyLinked` (links), `SameContent` (copies and renders)
  And already-set perms as `(chmod, skippedDue[AlreadySet])` / `(chown, skippedDue[AlreadySet])`
  And the active dry-run mode joins every skip reason list as `config.dryRun=delta` / `config.dryRun=all`
  And dry-run=all bypasses the zero-delta profile skip

Scenario: mutation lines report created or overwritten
  Status: tested
  When an op mutates a dest
  Then the line's action is `created` for a previously absent dest and `overwritten` for an existing one
  And template renders log under the `render-templates` scope

Scenario: each invoked command announces the profile it executes
  Status: tested
  When an os-mutating che command starts executing a profile
  Then an info line `<cmd>(runProfile): <profile>: [<op>(<delta>),...]` precedes the profile's op lines
  And the listed ops are those the command will run, deltas matching the discover log
  And ops wrapped by `run` log no runProfile line of their own

Scenario: rejected profiles log before the match lines
  Status: tested
  When discovery rejects profiles (runIf failed)
  Then each rejected profile logs its own info line `discover-profiles(noMatchDue[<failed condition>]): <profile>`, before the match lines
  And the reason is the actual condition that rejected it
  And remote profiles display as `remote:<reponame>:<profile>`
  And nothing rejected logs no line

Scenario: render-templates delta comes from a mock-render cache
  Status: tested
  When discovery counts render-templates deltas
  Then each template renders with mocked secret values and the output hash compares against the stored hash of the previous run
  And a differing or absent hash counts as delta
  And the cache stores the most recent hash only, keyed by dest, written on real renders only

---
Definitions: [Definitions.md](Definitions.md).
<!-- [<] 🤖🤖 -->
