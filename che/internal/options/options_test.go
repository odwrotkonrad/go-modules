package options

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestResolveBoolOr(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_bool_or.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		var user, spec *bool
		c.Input.Args.To(t, 0, &user)
		c.Input.Args.To(t, 1, &spec)
		o := &Options{}
		envVal := c.Context.Env["CHE_BOOL"]
		switch c.Context.Function {
		case "options.boolOr":
			return o.resolveBool("k", false, envVal, false, boolLayer{user, "config-file"}, boolLayer{spec, "specFile"}), nil
		case "options.boolDefaultTrue":
			return o.resolveBool("k", false, envVal, true, boolLayer{user, "config-file"}, boolLayer{spec, "specFile"}), nil
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
			return false, nil
		}
	})
}

type skipOpsGot struct {
	SkipOps    []string `yaml:"skipOps"`
	RunSkipOps []string `yaml:"allSkipOps"`
}

func TestResolveSkipOps(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_skip_ops.test.spec.yml", func(t *testing.T, c testyml.Case[skipOpsGot]) (skipOpsGot, error) {
		env := func(k string) string { return c.Context.Env[k] }
		o := Options{SkipOps: c.Input.Args.Strings(t, 0), RunSkipOps: c.Input.Args.Strings(t, 1)}
		var user, spec Layer
		c.Input.Args.To(t, 2, &user)
		c.Input.Args.To(t, 3, &spec)
		err := o.Resolve(env, user, spec)
		return skipOpsGot{SkipOps: o.SkipOps, RunSkipOps: o.RunSkipOps}, err
	})
}

// settingsWant is the resolve_settings expected.output shape.
type settingsWant struct {
	Delta       string   `yaml:"delta"`
	AllContains []string `yaml:"allContains"`
}

// settingsFlags decodes the case's flags arg into flag-layer option fields.
type settingsFlags struct {
	DryRun string `yaml:"dryRun"`
}

// TestResolveSettings: Resolve records every option's value + deciding source;
// SettingsDelta keeps only the non-default ones.
func TestResolveSettings(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_settings.test.spec.yml", func(t *testing.T, c testyml.Case[settingsWant]) {
		var flags settingsFlags
		var user, spec Layer
		c.Input.Args.To(t, 0, &flags)
		c.Input.Args.To(t, 1, &user)
		c.Input.Args.To(t, 2, &spec)
		o := &Options{DryRun: DryRunMode(flags.DryRun)}
		env := func(k string) string { return c.Context.Env[k] }
		require.NoError(t, o.Resolve(env, user, spec))
		if c.Expected.Output.Delta != "" {
			assert.Equal(t, c.Expected.Output.Delta, FormatSettings(o.SettingsDelta()))
		}
		all := FormatSettings(o.Settings)
		for _, want := range c.Expected.Output.AllContains {
			assert.Contains(t, all, want)
		}
	})
}

// [<] 🤖🤖
