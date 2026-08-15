package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestMethodKey(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/method_key.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return methodKey(c.Input.Args.String(t, 0)), nil
	})
}

func TestMethodMatches(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/method_matches.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return methodMatches(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestTargetHost(t *testing.T) {
	require.Equal(t, packages.Host{OS: "darwin", Arch: "arm64"}, targetHost(runCfg{mode: "with_no_deps", darwinVM: true}))
	require.Equal(t, packages.Host{OS: "linux", Arch: "amd64", Distro: "debian"}, targetHost(runCfg{mode: "with_no_deps", linuxArch: "amd64"}))
}

func TestEligibleMethods(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/eligible_methods.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		var entry packages.Entry
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &entry))
		got := eligibleMethods(entry, c.Input.Args.String(t, 1), c.Input.Args.Strings(t, 2))
		if got == nil {
			got = []string{}
		}
		return got, nil
	})
}

func TestSplitPackages(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/split_packages.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		got := splitPackages(c.Input.Args.String(t, 0))
		if got == nil {
			got = []string{}
		}
		return got, nil
	})
}

// [<] 🤖🤖
