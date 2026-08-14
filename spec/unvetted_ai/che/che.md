# Feature: Che Profile as a Unit of Execution

<!-- [>] 🤖🤖 -->

Scenario: profiles run exactly as the discovery log lists them
  Status: tested
  When a che command executes
  Then discovery order determines profile execution order

Scenario: a user wastes no time on profiles with nothing to change
  Status: tested
  When a command's ops over a profile carry no delta at all
  Then the profile is skipped wholesale, nothing executes
  And a debug line `will not run profile <ref>: no changes` announces the skip
  And config-empty op sets carry reason `options.skipOps` or `options.run.skipOps`, undefined ones `not defined`

Scenario: a user reads each profile's work as one uninterrupted block
  Status: tested
  When a che command executes over multiple profiles
  Then each profile completes all its discovered os-mutating operations before the next profile starts
  And operations of different profiles never interleave

Scenario: a heading anchors every op line to its profile
  Status: tested
  When a profile starts executing
  Then a `## Profile <ref>` heading announces it, before its ops log

Scenario: one dry-run banner opens the output, no per-line markers
  Status: tested
  When a che command executes with dry run enabled
  Then one line opens the whole output: `dry run (<mode>) no actual operations will be performed, <desc>`
  And delta's desc says only dests that would change report, all's that every dest reports its state
  And no other line carries a dry-run marker
  And `--dry-run=true` aliases delta mode

Scenario: a user sees what will run before watching it run
  Status: tested
  When a che command executes
  Then the discovery log precedes execution, at every log level that shows both

Scenario: script failures collect across the run by default
  Status: tested
  When a profile script fails and `--errexit` is not set
  Then the remaining scripts still run
  And the per-script status report lists every script as `ran` or `failed`
  And the run continues through the remaining ops and profiles, failures joining into the final error

Scenario: errexit aborts the run at the first script failure
  Status: tested
  When a profile script fails and `--errexit` (or `CHE_ERREXIT`) is set
  Then the remaining scripts never run
  And the remaining ops of the profile and the remaining profiles are skipped
  And the run exits nonzero, the error naming the failed script
<!-- [<] 🤖🤖 -->
