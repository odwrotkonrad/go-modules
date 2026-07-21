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
		if len(c.Expected.Output.AllContains) == 0 {
			assert.Equal(t, c.Expected.Output.Delta, FormatSettings(o.SettingsDelta()))
		}
		all := FormatSettings(o.Settings)
		for _, want := range c.Expected.Output.AllContains {
			assert.Contains(t, all, want)
		}
	})
}

// TestSettingsDisplay: config show labeling + sorting. An unset default shows
// "unset"; a source that set an option (even to its default value) shows the
// source and sorts with the changed options, ahead of the unset defaults, both
// groups in config order.
func TestSettingsDisplay(t *testing.T) {
	o := &Options{LogLevel: "info"} // [why] --log-level info: cliFlag set to the default value
	env := func(k string) string {
		if k == "CHE_VALIDATE_SPEC" {
			return "error"
		}
		return ""
	}
	require.NoError(t, o.Resolve(env, Layer{}, Layer{}))

	byKey := map[string]Setting{}
	for _, s := range o.Settings {
		byKey[s.Key] = s
	}
	assert.Equal(t, "cliFlag", byKey["logLevel"].DisplaySource(), "explicit-to-default keeps its source")
	assert.True(t, byKey["logLevel"].IsChanged(), "explicit-to-default counts as changed")
	assert.Equal(t, "env", byKey["validateSpec"].DisplaySource())
	assert.Equal(t, "unset", byKey["dryRun"].DisplaySource(), "no source -> unset, not default")
	assert.False(t, byKey["dryRun"].IsChanged())

	sorted := o.SettingsSorted()
	require.Len(t, sorted, len(o.Settings), "sorted holds every setting once")
	// changed group first, in config order; then unset, in config order.
	firstUnset := -1
	for i, s := range sorted {
		if !s.IsChanged() {
			firstUnset = i
			break
		}
	}
	require.Positive(t, firstUnset, "some changed settings sort first")
	for _, s := range sorted[:firstUnset] {
		assert.True(t, s.IsChanged(), "no unset setting before the boundary")
	}
	for _, s := range sorted[firstUnset:] {
		assert.False(t, s.IsChanged(), "no changed setting after the boundary")
	}
	assert.True(t, configOrderPreserved(o.Settings, sorted[:firstUnset]), "changed group keeps config order")
	assert.True(t, configOrderPreserved(o.Settings, sorted[firstUnset:]), "unset group keeps config order")
}

// configOrderPreserved reports whether subset appears in the same relative
// order as in the full Resolve-order settings list.
func configOrderPreserved(full, subset []Setting) bool {
	idx := map[string]int{}
	for i, s := range full {
		idx[s.Key] = i
	}
	for i := 1; i < len(subset); i++ {
		if idx[subset[i-1].Key] > idx[subset[i].Key] {
			return false
		}
	}
	return true
}

// [<] 🤖🤖
