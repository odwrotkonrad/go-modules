package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestResolveMethodKey(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_method_key.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return resolveMethodKey(c.Input.Args.String(t, 0)), nil
	})
}

func TestMatchesMethod(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/matches_method.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return matchesMethod(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1)), nil
	})
}

func TestResolveTargetHost(t *testing.T) {
	require.Equal(t, packages.Host{OS: "darwin", Arch: "arm64"}, resolveTargetHost(runCfg{mode: "with_no_deps", darwinVM: true}))
	require.Equal(t, packages.Host{OS: "linux", Arch: "amd64", Distro: "debian"}, resolveTargetHost(runCfg{mode: "with_no_deps", linuxArch: "amd64"}))
}

func TestResolveEligibleMethods(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_eligible_methods.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		var entry packages.Entry
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &entry))
		got := resolveEligibleMethods(entry, c.Input.Args.String(t, 1), c.Input.Args.Strings(t, 2))
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
