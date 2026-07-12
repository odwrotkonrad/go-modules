// Package config carries che's resolved runtime options: flag values with env fallbacks.
package config

// [>] 🤖🤖

// DryRunMode selects how a dry run reports: off (real run), delta (only dests
// that would change), all (every dest, as if nothing existed at the destination).
type DryRunMode int

// DryRun namespaces the DryRunMode values: config.DryRun.Off / .Delta / .All.
var DryRun = struct{ Off, Delta, All DryRunMode }{0, 1, 2}

// Config carries every resolved runtime option (flag value, env fallback applied).
type Config struct {
	Dir            string
	DryRun         DryRunMode
	Profile        string
	SkipExecIf     bool
	SkipPlugins    bool
	Debug          bool
	ValidateSchema string
}

// [<] 🤖🤖
