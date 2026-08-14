# Feature: Che Packages

<!-- [>] 🤖 -->

Scenario: a package about to install gets its base packages checked and installed first, as if listed in requires
  Status: tested
  When a package installs via a manager
  Then the `basePackages` groups (`common` plus the manager's own) are ensured first, each going through the normal install path like a `requires` entry
  And each group is ensured once per run, not per package

<!-- [<] 🤖 -->
