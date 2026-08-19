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
			return o.resolveBool("k", false, envVal, false, boolCandidate{user, "config-file"}, boolCandidate{spec, "specFile"}), nil
		case "options.boolDefaultTrue":
			return o.resolveBool("k", false, envVal, true, boolCandidate{user, "config-file"}, boolCandidate{spec, "specFile"}), nil
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
			return false, nil
		}
	})
}

type skipOpsGot struct {
	RunSkipOps []string `yaml:"runSkipOps"`
}

func TestResolveSkipOps(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_skip_ops.test.spec.yml", func(t *testing.T, c testyml.Case[skipOpsGot]) (skipOpsGot, error) {
		env := func(k string) string { return c.Context.Env[k] }
		o := Options{RunSkipOps: c.Input.Args.Strings(t, 0)}
		var user, spec Layer
		c.Input.Args.To(t, 1, &user)
		c.Input.Args.To(t, 2, &spec)
		err := o.Resolve(env, user, spec)
		return skipOpsGot{RunSkipOps: o.RunSkipOps}, err
	})
}

type settingsWant struct {
	Delta       string   `yaml:"delta"`
	AllContains []string `yaml:"allContains"`
}

type settingsFlags struct {
	DryRun string `yaml:"dryRun"`
}

func TestResolveSettings(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/resolve_settings.test.spec.yml", func(t *testing.T, c testyml.Case[settingsWant]) {
		var flags settingsFlags
		var user, spec Layer
		c.Input.Args.To(t, 0, &flags)
		c.Input.Args.To(t, 1, &user)
		c.Input.Args.To(t, 2, &spec)
		o := &Options{DryRun: DryRunMode(flags.DryRun)}
		env := func(k string) string { return c.Context.Env[k] }
		if c.Expected.Check(t, o.Resolve(env, user, spec)) {
			return
		}
		if len(c.Expected.Output.AllContains) == 0 {
			assert.Equal(t, c.Expected.Output.Delta, FormatSettings(o.SettingsDelta()))
		}
		all := FormatSettings(o.Settings)
		for _, want := range c.Expected.Output.AllContains {
			assert.Contains(t, all, want)
		}
	})
}

func TestResolveSilenceInstallStdout(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_silence_install_stdout.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		o := &Options{LogLevel: c.Input.Args.String(t, 1), PackagesSilenceInstallStdout: c.Input.Args.String(t, 0)}
		env := func(k string) string { return c.Context.Env[k] }
		if err := o.Resolve(env, Layer{}, Layer{}); err != nil {
			return "", err
		}
		return o.PackagesSilenceInstallStdout, nil
	})
}

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
