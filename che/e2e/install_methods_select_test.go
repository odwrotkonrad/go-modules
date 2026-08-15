package e2e

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

func TestMethodKey(t *testing.T) {
	require.Equal(t, "brew/formulae", methodKey("brew"))
	require.Equal(t, "brew/cask", methodKey("cask"))
	require.Equal(t, "apt", methodKey("apt"))
	require.Equal(t, "binariesRemoteArchive", methodKey("binariesRemoteArchive"))
}

func TestMethodMatches(t *testing.T) {
	cases := []struct {
		name, key, selected string
		want                bool
	}{
		{name: "empty selects everything", key: "apt", selected: "", want: true},
		{name: "all selects everything", key: "brew/cask", selected: "all", want: true},
		{name: "exact", key: "apt", selected: "apt", want: true},
		{name: "exact slashed", key: "brew/cask", selected: "brew/cask", want: true},
		{name: "prefix", key: "brew/formulae", selected: "brew", want: true},
		{name: "prefix other sub", key: "brew/cask", selected: "brew", want: true},
		{name: "no partial word", key: "brewx", selected: "brew", want: false},
		{name: "mismatch", key: "apt", selected: "brew", want: false},
		{name: "selected deeper", key: "brew", selected: "brew/cask", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, methodMatches(c.key, c.selected))
		})
	}
}

func TestTargetHost(t *testing.T) {
	require.Equal(t, packages.Host{OS: "darwin", Arch: "arm64"}, targetHost(runCfg{mode: "with_no_deps", darwinVM: true}))
	require.Equal(t, packages.Host{OS: "linux", Arch: "amd64", Distro: "debian"}, targetHost(runCfg{mode: "with_no_deps", linuxArch: "amd64"}))
}

func TestEligibleMethods(t *testing.T) {
	entry := packages.Entry{Items: []packages.Item{
		{Mgr: "brew"}, {Mgr: "cask"}, {Mgr: "apt"}, {Mgr: "binariesRemoteArchive"}, {Mgr: "binariesRemoteArchive"},
	}}
	darwin := []string{"brew", "brew/cask", "binariesRemoteArchive", "script", "npm", "go", "gem", "pyenv", "nvm"}
	linux := []string{"apt", "binariesRemoteArchive", "script", "npm", "go", "gem", "pyenv", "nvm"}
	cases := []struct {
		name, method string
		eligible     []string
		want         []string
	}{
		{name: "darwin drops apt", eligible: darwin, want: []string{"brew", "cask", "binariesRemoteArchive"}},
		{name: "linux drops brew and cask", eligible: linux, want: []string{"apt", "binariesRemoteArchive"}},
		{name: "method narrows further", method: "brew", eligible: darwin, want: []string{"brew", "cask"}},
		{name: "method x platform mismatch", method: "apt", eligible: darwin, want: nil},
		{name: "dedupes repeated managers", method: "binariesRemoteArchive", eligible: linux, want: []string{"binariesRemoteArchive"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, eligibleMethods(entry, c.method, c.eligible))
		})
	}
}

func TestSplitPackages(t *testing.T) {
	require.Equal(t, []string{"jq"}, splitPackages("jq"))
	require.Equal(t, []string{"jq", "podman"}, splitPackages("jq,podman"))
	require.Equal(t, []string{"jq", "podman"}, splitPackages(" jq , podman ,"))
	require.Empty(t, splitPackages(",,"))
}

// [<] 🤖🤖
