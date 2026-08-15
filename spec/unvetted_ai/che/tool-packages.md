# Feature: Tool-Scoped Packages (toolPackages)

<!-- [>] 🤖🤖 -->

Packages that live inside a host tool rather than on PATH (vscode extensions
today, pip/npm libraries later) are declared under a top-level `toolPackages:`
section of packages.yml, keyed by tool, each entry a package name mapping to a
version pin (null/empty value: rolling). Profiles select them via
`include.installToolPackages`, the CLI via `che packages install --kind=<tool>`.

Scenario: a packages file declares tool-scoped packages per host tool
  Status: tested
  When packages.yml carries `toolPackages.<tool>` with `name: pin` and bare `name:` entries
  Then a bare/null value means rolling, a scalar pins the version
  And an unknown tool key is a parse error listing the known tools
  And overrides merge per tool entry and `che packages config --delta` diffs per tool entry

Scenario: a profile installs tool packages next to regular packages
  Status: tested
  When a profile includes `installToolPackages: {vscode: [name, {name, version}]}`
  Then the install-packages op installs them after the profile's regular packages
  And included profiles compose per tool, later refs re-pin by name, `exclude.installToolPackages` drops names per tool
  And after a real run their presence is checked via the tool's own listing

Scenario: a user installs tool packages directly via --kind
  Status: tested
  When I run `che packages install --kind=vscode <name...>`
  Then each name resolves in `toolPackages.vscode` (an unknown name is a hard error naming the file)
  And a present package with a matching pin skips, a drifted pin reinstalls, `--update` refreshes unpinned ones
  And dry-run emits the would-be installs without running the tool
  And `che packages check-present --kind=vscode` errors on missing entries, defaulting to the profiles' selection, else the whole section

Scenario: the tool's own base packages install first, an absent tool skips with a warning
  Status: tested
  When a tool package installs and `basePackages.<tool>` names the tool's carrier package (vscode: code)
  Then the carrier installs first through the regular pipeline
  And if the tool command is still absent the tool's packages skip with a warning instead of erroring

<!-- [<] 🤖🤖 -->
