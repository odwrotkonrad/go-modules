package config

// [>] 🤖🤖

// Domain model:
//
//	Config  CLI option values (cobra flag destinations), finalized in place by
//	        Resolve: env fallbacks (flags win), validated mode values

// DryRunMode selects how a dry run reports: off (real run), delta (only dests
// that would change), all (every dest, as if nothing existed at the destination).
type DryRunMode string

// DryRun namespaces the DryRunMode values: config.DryRun.Off / .Delta / .All.
var DryRun = struct{ Off, Delta, All DryRunMode }{"", "delta", "all"}

// ValidateSpecMode selects how che.yml schema violations report: warn (log)
// or error (abort). Empty resolves to warn.
type ValidateSpecMode string

// ValidateSpec namespaces the ValidateSpecMode values.
var ValidateSpec = struct{ Warn, Error ValidateSpecMode }{"warn", "error"}

// Config carries every runtime option.
type Config struct {
	Dir               string
	DryRun            DryRunMode
	ValidateSpec      ValidateSpecMode
	Profile           string
	SkipExecIf        bool
	SkipPlugins       bool
	Debug             bool
	RenderSkipSecrets bool
}

// [<] 🤖🤖
