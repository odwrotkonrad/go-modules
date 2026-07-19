# Feature: Che Profile as a Unit of Execution

<!-- [>] 🤖🤖 -->

Scenario: profiles execute in discovery order
  Status: tested
  When a che command executes
  Then discovery order determines profile execution order

Scenario: zero-delta profiles are skipped
  Status: tested
  When a command's ops over a profile carry no delta at all
  Then the profile is skipped wholesale, nothing executes
  And the announce reads `<cmd>(runProfile, skippedDue[NoDelta])`, config-empty op sets `skippedDue[config.skipOps]`, undefined ones `skippedDue[NoDef]`

Scenario: profiles execute one after another
  Status: tested
  When a che command executes over multiple profiles
  Then each profile completes all its discovered os-mutating operations before the next profile starts
  And operations of different profiles never interleave

Scenario: each che profile execution announces itself
  Status: tested
  When a profile starts executing
  Then an info line announces the profile, before its commands log

Scenario: dry run announces itself once
  Status: tested
  When a che command executes with dry run enabled
  Then one line opens the whole output: `dry-run(config.dryRun=<mode>): <desc>`
  And delta's desc says only dests that would change report, all's that every dest reports its state
  And no other line carries a dry-run marker
  And `--dry-run=true` aliases delta mode

Scenario: discovery log precedes execution
  Status: tested
  When a che command executes
  Then the discovery log precedes execution, regardless of debug mode
<!-- [<] 🤖🤖 -->
