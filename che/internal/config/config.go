package config

// [>] 🤖🤖

// DryRunMode selects how a dry run reports: off (real run), delta (only dests
// that would change), all (every dest, as if nothing existed at the destination).
type DryRunMode int

const (
	DryRunOff DryRunMode = iota
	DryRunDelta
	DryRunAll
)

// Option keys one runtime option, named after its flag.
type Option string

const (
	OptionDir         Option = "directory"
	OptionDryRun      Option = "dry-run"
	OptionProfile     Option = "profile"
	OptionSkipExecIf  Option = "skip-exec-if"
	OptionSkipPlugins Option = "skip-plugins"
	OptionDebug       Option = "debug"
)

// Config carries every resolved runtime option (flag value, env fallback applied).
type Config struct {
	Dir         string
	DryRun      DryRunMode
	Profile     string
	SkipExecIf  bool
	SkipPlugins bool
	Debug       bool
}

// value maps opt to its field. Unknown opt is a programming error: panic.
func (c Config) value(opt Option) any {
	switch opt {
	case OptionDir:
		return c.Dir
	case OptionDryRun:
		return c.DryRun
	case OptionProfile:
		return c.Profile
	case OptionSkipExecIf:
		return c.SkipExecIf
	case OptionSkipPlugins:
		return c.SkipPlugins
	case OptionDebug:
		return c.Debug
	}
	panic("unknown option: " + string(opt))
}

// IsOptionEqualTo reports whether opt's resolved value equals val.
func (c Config) IsOptionEqualTo(opt Option, val any) bool { return c.value(opt) == val }

// [<] 🤖🤖
