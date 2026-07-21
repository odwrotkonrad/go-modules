# Feature: Che Backup Operation

<!-- [>] 🤖🤖 -->

`backup`: archive every existing op dest that would change (unsettled links,
differing copies, differing renders) into the per-run backup archive.

Scenario: backup runs before the other ops in run
  Status: tested
  When `run` executes a profile
  Then the backup stage runs after init-remote-sources and discover-profiles, before every other op
  And it archives every existing dest that would change into one per-run archive

Scenario: ops wrapped by run do not back up individually
  Status: tested
  When `run` executes its wrapped ops
  Then no wrapped op writes its own backup archive

Scenario: wrapped ops' ledger records point at the run backup archive
  Status: implemented
  When a wrapped op records a mutation
  Then the record's backup reference is the run's backup archive

Scenario: backup logs its delta summary and the created archive
  Status: tested
  When backup archives
  Then a `backup delta <op> (<n> changes), <op> (<n> changes)` line always lists the covered file ops with their deltas
  And a `created <size>, <path>` line reports the written archive
  And nothing to back up writes and logs nothing more
  And dry run writes no archive, predicting `will not create <path>: <dry-run reason>` instead

Scenario: standalone backup archives only dests that would change
  Status: tested
  When I invoke `backup` standalone
  Then every existing dest an op would change archives into the per-run backup archive
  And settled dests are not archived
  And nothing to change archives nothing

Scenario: direct op subcommands still back up their own dests
  Status: tested
  When I invoke an os-mutating op subcommand directly, not wrapped by `run`
  Then the op archives its own dests before mutating, as before

Scenario: the backup stage announces as an op heading
  Status: tested
  When the backup stage starts within `run`
  Then a `### backup` heading announces it under the profile heading
  And the delta line always logs beneath it, even with nothing to back up

<!-- [<] 🤖🤖 -->
