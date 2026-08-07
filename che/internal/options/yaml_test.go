package options

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

func TestDefaultSettingsYAMLIsSeedableConfig(t *testing.T) {
	settings, err := DefaultSettings()
	require.NoError(t, err)
	out, err := SettingsYAML(settings)
	require.NoError(t, err)

	for _, want := range []string{
		"logLevel: info",
		"checkInPath: true",
		"autoDiscover: true",
		"installDestinationCandidates: [~/.local/bin, ~/bin]",
		"installDestinationCandidates: [~/.local/share/zsh/site-functions, ~/.zfunc]",
		"skipOps: []",
	} {
		require.Contains(t, out, want)
	}
	for _, unwanted := range []string{"cheWorkingDirectory", "skipRunIf", "errexit", "override", "dryRun: \"false\""} {
		require.NotContains(t, out, unwanted)
	}

	var layer spec.Options
	require.NoError(t, yaml.Unmarshal([]byte(out), &layer))
	o := &Options{}
	require.NoError(t, o.Resolve(func(string) string { return "" }, layer, Layer{}))
	require.Equal(t, DryRun.Off, o.DryRun)
	require.Equal(t, "info", o.LogLevel)
}

func TestSettingsYAMLNestsAndTypes(t *testing.T) {
	out, err := SettingsYAML([]Setting{
		{Key: "logLevel", Value: "info"},
		{Key: "packages.binary.checkInPath", Value: "true", Kind: "bool"},
		{Key: "packages.preferredInstallationMethods", Value: "binary,brew", Kind: "list"},
		{Key: "otel.enabled", Value: "false", Kind: "bool"},
	})
	require.NoError(t, err)
	var got struct {
		LogLevel string `yaml:"logLevel"`
		Packages struct {
			Binary struct {
				CheckInPath bool `yaml:"checkInPath"`
			} `yaml:"binary"`
			Preferred []string `yaml:"preferredInstallationMethods"`
		} `yaml:"packages"`
		Otel struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"otel"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(out), &got))
	require.Equal(t, "info", got.LogLevel)
	require.True(t, got.Packages.Binary.CheckInPath)
	require.Equal(t, []string{"binary", "brew"}, got.Packages.Preferred)
	require.False(t, got.Otel.Enabled)
}

// [<] 🤖🤖
