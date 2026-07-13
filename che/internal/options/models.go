package options

// [>] 🤖🤖

// Domain model:
//
//	Options    CLI option values (cobra flag destinations), finalized in place
//	           by Resolve: flags win over env vars, env vars over the local
//	           che.yml options: block (SpecLayer), then defaults
//	SpecLayer  the local che.yml options: knobs feeding Resolve

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
	Dir               string
	WorkingDirectory  string
	DryRun            DryRunMode
	ValidateSpec      ValidateSpecMode
	Profile           string
	SkipExecIf        bool
	SkipRemoteRefs    bool
	Debug             bool
	RenderSkipSecrets bool
}

// SpecLayer is the local spec's options: contribution to Resolve: applied
// under flags and env vars, over the defaults.
type SpecLayer struct {
	ValidateSpec string
	Debug        *bool
}

// [<] 🤖🤖
