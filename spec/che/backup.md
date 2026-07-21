# Feature: Che Backup Operation

<!-- [>] 🤖🤖 -->

`backup` archives every existing op dest that would change (unsettled links,
differing copies, differing renders) into the per-run backup archive. It has
three subcommands: `backup create` (archive), `backup restore` (restore state
by `--run-id`, `--backup-id`, or `--timestamp`), `backup ls` (list the backup
points).

Scenario: backup has three subcommands
  Status: tested
  When I invoke `backup` with a subcommand
  Then `backup create` archives every would-change dest into a per-run archive
  And `backup restore` restores state by `--run-id` (that run's archives), `--backup-id` (one archive), or `--timestamp` (files present at that time)
  And `backup ls` lists the backup points
  And bare `backup` with no subcommand prints usage listing the three

Scenario: backup archives are laid out by profile and op
  Status: tested
  When any backup archive is written
  Then its path is `backups/<profile-slug>/<op>/<ts>-<backup-id>.tar.bz2` under the state dir
  And the profile ref is slugified into the path segment (`cli/macos` -> `cli-macos`)
  And every archive of one run shares the run's timestamp and run id (the ledger run key)
  And each archive carries its own unique 12-char backup id

Scenario: backup create archives would-change dests
  Status: tested
  When I invoke `backup create`
  Then it archives every existing dest an op would change into one per-run archive
  And settled dests are not archived
  And nothing to change archives nothing
  And it is the default archive action the run stage and direct ops invoke

Scenario: backup ls lists the backup points
  Status: tested
  When I invoke `backup ls`
  Then it lists each ledger-recorded backup point under a `# backups` heading
  And each entry shows the run id, backup id, timestamp, size, and abbreviated path
  And the newest backup point lists first
  And no backup points lists nothing

Scenario: backup restore by run id restores that run's archives
  Status: tested
  When I invoke `backup restore --run-id <id>`
  Then it restores that run's backup archives exactly
  And a run id matching no run fails with a clear error

Scenario: backup restore by backup id restores one archive
  Status: tested
  When I invoke `backup restore --backup-id <id>`
  Then it restores the single archive whose filename carries that backup id
  And a backup id matching no archive fails with a clear error

Scenario: backup restore by timestamp is a point-in-time restore
  Status: tested
  When I invoke `backup restore --timestamp <ts>`
  Then it performs a point-in-time restore: each dest to the most recent backup at or before the timestamp
  And a dest with no backup at or before the timestamp is left as-is
  And a timestamp before every backup fails with a clear error (nothing to restore)

Scenario: backup restore restores an archive onto its dests
  Status: tested
  When I invoke `backup restore` with a selector matching a known archive
  Then it restores every entry in the archive back onto its recorded dest
  And exactly one of `--run-id`, `--backup-id`, `--timestamp` must be passed, else a clear error
  And a dest that drifted from che's last recorded state is skipped, not clobbered
  And dry run reports each restore as `restore <dest> (dry run)`, writing nothing
  And an unreadable archive fails with a clear error

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
  And dry run writes no archive, predicting `create <path> (dry run)` instead

Scenario: standalone backup archives only dests that would change
  Status: tested
  When I invoke `backup create` standalone
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
  Then a `## backup` heading announces it under the profile heading
  And the delta line always logs beneath it, even with nothing to back up

<!-- [<] 🤖🤖 -->
