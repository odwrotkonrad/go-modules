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

// Options carries every runtime option.
type Options struct {
	Dir              string
	WorkingDirectory string
	DryRun           DryRunMode
	ValidateSpec     ValidateSpecMode
	// ValidateSpecCLI is the flag/env/user-config validateSpec ("" if none set),
	// overriding each spec's own options.validateSpec per-spec.
	ValidateSpecCLI   ValidateSpecMode
	Profiles          []string
	SkipExecIf        bool
	SkipRemoteRefs    bool
	Debug             bool
	RenderSkipSecrets bool
	// AutoDiscover is the user-config global default for profiles that set
	// neither profile nor spec autoDiscover (nil: unset).
	AutoDiscover *bool
	Otel         Otel
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
}

// Layer is one options: contribution feeding Resolve (the user-config file or
// the local spec's options: block), sharing spec.Options' shape.
type Layer = spec.Options

// [<] 🤖🤖
