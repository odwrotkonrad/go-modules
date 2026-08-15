package options

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestDefaultSettingsYAMLIsSeedableConfig(t *testing.T) {
	settings, err := DefaultSettings()
	require.NoError(t, err)
	out, err := SettingsYAML(settings)
	require.NoError(t, err)

	var layer spec.Options
	require.NoError(t, yaml.Unmarshal([]byte(out), &layer))
	o := &Options{}
	require.NoError(t, o.Resolve(func(string) string { return "" }, layer, Layer{}))
	require.Equal(t, DryRun.Off, o.DryRun)
	require.Equal(t, "info", o.LogLevel)
}

type settingArg struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
	Kind  string `yaml:"kind"`
}

func TestSettingsYAML(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/settings_yaml.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]any]) {
		settings, err := DefaultSettings()
		require.NoError(t, err)
		if !c.Input.Args.Bool(t, 0) {
			var args []settingArg
			c.Input.Args.To(t, 1, &args)
			settings = settings[:0]
			for _, a := range args {
				settings = append(settings, Setting{Key: a.Key, Value: a.Value, Kind: a.Kind})
			}
		}
		out, err := SettingsYAML(settings)
		require.NoError(t, err)
		for _, m := range c.Expected.StdOut {
			testyml.MustMatch(t, out, m)
		}
		for _, m := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, out, m)
		}
		if len(c.Expected.Output) > 0 {
			var got map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(out), &got))
			assert.Equal(t, c.Expected.Output, got)
		}
	})
}

// [<] 🤖🤖
