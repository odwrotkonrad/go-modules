# Feature: Install Stdout Silencing

<!-- [>] 🤖🤖 -->

At `CHE_LOG_LEVEL=info` (the default) installation-method stdout is noise:
apt/brew/npm progress drowns che's own log lines. Silenced by default,
`--silence-install-stdout` on `che packages install` controls it. Error output
always stays visible, debug level shows everything.

Scenario: at info level the installation method's normal output is silenced
  Status: tested
  Given `CHE_LOG_LEVEL=info` (the default)
  When `che packages install` runs an installation method (apt, brew, npm, ...)
  Then the method's normal stdout is not printed
  And che's own log lines (install, installed, skip, warn) still print

Scenario: a failing installation method's output is always visible
  Status: tested
  Given any log level and any `--silence-install-stdout` value
  When an installation method exits non-0
  Then its captured output prints with the error
  And the failure is attributable without rerunning at debug

Scenario: --silence-install-stdout on the install command overrides the default
  Status: tested
  When I invoke `che packages install --silence-install-stdout=false`
  Then the method's stdout streams even at info level
  And `--silence-install-stdout` (or `=true`) silences it even at debug level

Scenario: at debug level the installation method's output is visible
  Status: tested
  Given `CHE_LOG_LEVEL=debug` (or `trace`)
  When `che packages install` runs an installation method
  Then the method's stdout streams as it runs

Scenario: e2e packages and e2e install-methods tests keep the plain info behavior
  Status: tested
  When `make e2e-packages` or `make e2e-install-methods` runs (CHE_LOG_LEVEL=info)
  Then che runs with the default silencing, no flag and no special-casing
  And the tests assert against che's own log lines, not the methods' stdout

<!-- [<] 🤖🤖 -->
