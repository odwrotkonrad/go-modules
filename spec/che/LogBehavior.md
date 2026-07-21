# Feature: Che Discover Log Content

<!-- [>] 🤖🤖 -->

What the discover and op logs report, independent of line format (the human/
machine split and level gating live in
[LogRedesignBehavior.md](LogRedesignBehavior.md)). Discovery reports each
profile, its working directory, and its ops with delta counts; ops report the
mutations they make or would make.

Scenario: a discovered profile reports its working directory and ops
  Status: tested
  When discovery reports a profile
  Then it lists the profile's working directory
  And it lists the os-mutating che commands the profile would perform, execution order
  And config-skipped ops (--skip-ops) are excluded
  And each op carries its delta count (operations that would change os state now)
  And at debug each op additionally carries its all count (os-mutating operations declared)

Scenario: dry run reports the true predicted operations
  Status: tested
  When dry-run=delta runs
  Then only operations that would change os state report, as their real action
  And dry-run=all additionally reports every settled dest, with its skip reason
  And skip reasons distinguish already-exists (dirs), already-linked (links), same-content (copies and renders), already-set (perms)
  And the active dry-run mode joins every skip reason
  And dry-run=all bypasses the zero-delta profile skip

Scenario: mutation reports created or overwritten
  Status: tested
  When an op mutates a dest
  Then the action is created for a previously absent dest and overwritten for an existing one
  And template renders report under the render-templates scope

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
  And at debug its announce marks that nothing would change

<!-- [<] 🤖🤖 -->
