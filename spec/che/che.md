# Feature: Che Profile as a Unit of Execution

<!-- [>] 🤖🤖 -->

Scenario: a user predicts execution order from the discovery log, profiles run exactly as listed
  Status: tested
  When a che command executes
  Then discovery order determines profile execution order

Scenario: a user wastes no time on profiles with nothing to change
  Status: tested
  When a command's ops over a profile carry no delta at all
  Then the profile is skipped wholesale, nothing executes
  And a debug line `will not run profile <ref>: no changes` announces the skip
  And config-empty op sets carry reason `options.skipOps` or `options.run.skipOps`, undefined ones `not defined`

Scenario: a user reads each profile's work as one uninterrupted block, never interleaved with another's
  Status: tested
  When a che command executes over multiple profiles
  Then each profile completes all its discovered os-mutating operations before the next profile starts
  And operations of different profiles never interleave

Scenario: an operator knows which profile produced every op line, a heading anchors each
  Status: tested
  When a profile starts executing
  Then a `## Profile <ref>` heading announces it, before its ops log

Scenario: a user trusts one opening banner that nothing will change, the rest of the output stays clean
  Status: tested
  When a che command executes with dry run enabled
  Then one line opens the whole output: `dry run (<mode>) no actual operations will be performed, <desc>`
  And delta's desc says only dests that would change report, all's that every dest reports its state
  And no other line carries a dry-run marker
  And `--dry-run=true` aliases delta mode

Scenario: a user sees what will run before watching it run, at every log level showing both
  Status: tested
  When a che command executes
  Then the discovery log precedes execution, at every log level that shows both
<!-- [<] 🤖🤖 -->
