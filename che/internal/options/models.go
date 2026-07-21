package options

// [>] 🤖🤖

import "gitlab.com/konradodwrot/go-modules/che/internal/spec"

// Domain model:
//
//	Options  CLI option values (cobra flag destinations), finalized in place by
//	         Resolve. Per-field precedence, most specific wins: flags > env vars
//	         > the user-config file ($XDG_CONFIG_HOME/che/config.yml) > the local
//	         che.yml options: block > defaults. The user-config and spec layers
//	         share one shape, spec.Options.

// DryRunMode selects how a dry run reports: off (real run), delta (only dests
// that would change), all (every dest, as if nothing existed at the destination).
type DryRunMode string

// DryRun namespaces the DryRunMode values: options.DryRun.Off / .Delta / .All.
var DryRun = struct{ Off, Delta, All DryRunMode }{"", "delta", "all"}

// ValidateSpecMode selects how che.yml schema violations report: warn (log)
// or error (abort). Empty resolves to warn.
type ValidateSpecMode string

// ValidateSpec namespaces the ValidateSpecMode values.
var ValidateSpec = struct{ Warn, Error ValidateSpecMode }{"warn", "error"}

// OpNames lists every op name in run order: the canonical set skip-ops
// values validate against (cobra subcommand wiring lives in cli.ops).
var OpNames = []string{"prune-broken-links", "make-dirs", "make-links", "make-copies", "render-templates", "run-scripts"}

// Options carries every runtime option.
type Options struct {
	// CheWorkingDirectory is -C: chdir here before resolving the repo.
	CheWorkingDirectory string
	// ProfileWorkingDirectory is the che-level default load-ops source tree
	// (spec/profile options.profileWorkingDirectory override it).
	ProfileWorkingDirectory string
	DryRun                  DryRunMode
	ValidateSpec            ValidateSpecMode
	// ValidateSpecCLI is the flag/env/user-config validateSpec ("" if none set),
	// overriding each spec's own options.validateSpec per-spec.
	ValidateSpecCLI ValidateSpecMode
	Profiles        []string
	// SkipOps skips ops everywhere: dropped from the all sequence, direct op
	// subcommands become logged no-ops.
	SkipOps []string
	// RunSkipOps skips ops in the run sequence only.
	RunSkipOps     []string
	SkipRunIf      bool
	SkipRemoteRefs bool
	// LogLevel selects the human-log verbosity: error | warn | info | debug |
	// trace (default info).
	LogLevel          string
	RenderSkipSecrets bool
	// AutoDiscover is the auto-discovery master switch (default true): whether
	// profiles marked autoDiscover run on bare che; false leaves only
	// --profiles and include.profiles refs.
	AutoDiscover bool
	Otel         Otel
	// Settings is the resolution report: every option's final value and the
	// layer that decided it, in Resolve order (the config log renders it).
	Settings []Setting
}

// Otel is the resolved OTLP telemetry config: the finalized runtime shape the
// telemetry provider reads. Disabled -> the provider is a no-op. The local
// collector is plaintext, so the client always dials without TLS.
type Otel struct {
	Enabled  bool
	Endpoint string
	Protocol string // grpc | http
	Metrics  bool
	Logs     bool
	Traces   bool
}

// Layer is one options: contribution feeding Resolve (the user-config file or
// the local spec's options: block), sharing spec.Options' shape.
type Layer = spec.Options

// [<] 🤖🤖
